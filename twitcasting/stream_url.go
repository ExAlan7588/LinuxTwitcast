package twitcasting

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/jmoiron/jsonq"

	"github.com/jzhang046/croned-twitcasting-recorder/record"
)

const (
	baseDomain     = "https://twitcasting.tv"
	apiEndpoint    = baseDomain + "/streamserver.php"
	requestTimeout = 4 * time.Second
	userAgent      = "Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36"
)

var httpClient = &http.Client{
	Timeout: requestTimeout,
}

var (
	ErrStreamOffline    = errors.New("live stream is offline")
	ErrPasswordRequired = errors.New("live stream requires a password")
	ErrStreamerNotFound = errors.New("streamer screen-id was not found")
	ErrMemberOnlyLive   = errors.New("live stream is members-only")
)

var (
	twitterProfileSizeSuffixRegex = regexp.MustCompile(`_(normal|bigger|mini)(\.[A-Za-z0-9]+)$`)
	genericSmallImageSuffixRegex  = regexp.MustCompile(`-s(\.[A-Za-z0-9]+)$`)
	titleTagRegex                 = regexp.MustCompile(`(?is)<title>(.*?)</title>`)
	authorMetaRegex               = regexp.MustCompile(`(?is)<meta\s+name="author"\s+content="([^"]+)"`)
	twitterTitleMetaRegex         = regexp.MustCompile(`(?is)<meta\s+name="twitter:title"\s+content="([^"]*)"`)
	twitterDescriptionMetaRegexes = compileMetaContentRegexes("name", "twitter:description")
	movieTitleMetaRegexes         = append(
		compileMetaContentRegexes("name", "twitter:title"),
		compileMetaContentRegexes("property", "og:title")...,
	)
	liveTitleHeadingRegex    = regexp.MustCompile(`(?is)<div[^>]+\bclass=["'][^"']*\btw-player-page-title-title\b[^"']*["'][^>]*>.*?<h2>(.*?)</h2>`)
	broadcasterNameDataRegex = regexp.MustCompile(`(?is)\bdata-broadcaster-name=["']([^"'<>]+)["']`)
	pageMovieIDDataRegex     = regexp.MustCompile(`(?is)\bdata-movie-id=["'](\d+)["']`)
	htmlTagRegex             = regexp.MustCompile(`(?is)<[^>]+>`)
	avatarURLRegexes         = []*regexp.Regexp{
		regexp.MustCompile(`(?is)\bdata-broadcaster-profile-image=["']([^"'<>]+)["']`),
		regexp.MustCompile(`(?is)<img[^>]+\bclass=["'][^"']*\bauthorthumbnail\b[^"']*["'][^>]+\bsrc=["']([^"'<>]+)["']`),
		regexp.MustCompile(`(?is)<img[^>]+\bsrc=["']([^"'<>]+)["'][^>]+\bclass=["'][^"']*\bauthorthumbnail\b[^"']*["']`),
		regexp.MustCompile(`(?is)<div[^>]+\bclass=["'][^"']*\btw-user-nav2-icon\b[^"']*["'][^>]*>\s*<img[^>]+\bsrc=["']([^"'<>]+)["']`),
		regexp.MustCompile(`(?is)<img[^>]+\bclass=["'][^"']*\btw-unit-own-member-icon\b[^"']*["'][^>]+\bsrc=["']([^"'<>]+)["']`),
	}
	ogImageMetaRegexes      = compileMetaContentRegexes("property", "og:image")
	twitterImageMetaRegexes = compileMetaContentRegexes("name", "twitter:image")
	filenameSanitizer       = strings.NewReplacer(
		"\\", "＼",
		"/", "／",
		":", "：",
		"*", "＊",
		"?", "？",
		"\"", "”",
		"<", "＜",
		">", "＞",
		"|", "｜",
	)
	membershipMarkers = []string{
		"members-only stream",
		"member-only stream",
		"members only stream",
		"member only stream",
		"members-only",
		"メンバーシップ限定配信",
		"メンバー限定配信",
		"會員限定直播",
		"会员限定直播",
		"會員限定配信",
		"会员限定配信",
		"membershipjoinplans.php?u=",
		"membershipjoindetail.php?u=",
	}
)

type streamLookupError struct {
	err    error
	result record.StreamLookupResult
}

func (e *streamLookupError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *streamLookupError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func wrapStreamLookupError(err error, result record.StreamLookupResult) error {
	if err == nil {
		return nil
	}
	return &streamLookupError{err: err, result: result}
}

func LookupResultFromError(err error) (record.StreamLookupResult, bool) {
	var target *streamLookupError
	if errors.As(err, &target) && target != nil {
		return target.result, true
	}
	return record.StreamLookupResult{}, false
}

// 轮询 TwitCasting 时，离线和会员限定都是常态，不该继续伪装成 Error 噪音。
func LogStreamLookupOutcome(streamer string, lookup record.StreamLookupResult, err error) {
	switch {
	case err == nil:
		log.Printf("[Info] Fetched stream URL for streamer [%s]: %s\n", streamer, lookup.StreamURL)
	case errors.Is(err, ErrStreamOffline):
		log.Printf("[Info] Streamer [%s] is currently offline; skipping this polling round\n", streamer)
	case errors.Is(err, ErrMemberOnlyLive):
		title := strings.TrimSpace(lookup.Title)
		if title == "" {
			log.Printf("[Info] Streamer [%s] is currently members-only; recording stays skipped\n", streamer)
			return
		}
		log.Printf("[Info] Streamer [%s] is currently members-only; recording stays skipped (%s)\n", streamer, title)
	case errors.Is(err, ErrPasswordRequired):
		log.Printf("[Warn] Streamer [%s] requires a TwitCasting password before recording can continue\n", streamer)
	case errors.Is(err, ErrStreamerNotFound):
		log.Printf("[Warn] Streamer [%s] screen-id was not found during lookup\n", streamer)
	default:
		log.Printf("[Error] Failed fetching stream URL for streamer [%s]: %v\n", streamer, err)
	}
}

type streamPageInfo struct {
	streamerName     string
	title            string
	avatarURL        string
	coverURL         string
	displayedMovieID string
	membershipPage   bool
	passwordRequired bool
	memberOnly       bool
}

type StreamerProfile struct {
	ScreenID         string
	StreamerName     string
	Title            string
	AvatarURL        string
	PasswordRequired bool
}

func GetWSStreamUrl(streamer string) (record.StreamLookupResult, error) {
	return GetWSStreamUrlWithPassword(streamer, "")
}

// 新增直播主时只需要确认主页存在并能取到基础资料，不要求对方当前正在直播。
func LookupStreamerProfile(streamer string) (StreamerProfile, error) {
	screenID := strings.TrimSpace(streamer)
	if screenID == "" {
		return StreamerProfile{}, errors.New("screen-id is required")
	}

	pageInfo, statusCode, err := fetchStreamInfoWithStatus(screenID)
	if err != nil {
		return StreamerProfile{}, err
	}
	if statusCode == http.StatusNotFound {
		return StreamerProfile{}, ErrStreamerNotFound
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return StreamerProfile{}, fmt.Errorf("unexpected TwitCasting status: %d", statusCode)
	}

	return StreamerProfile{
		ScreenID:         screenID,
		StreamerName:     pageInfo.streamerName,
		Title:            pageInfo.title,
		AvatarURL:        pageInfo.avatarURL,
		PasswordRequired: pageInfo.passwordRequired,
	}, nil
}

func GetWSStreamUrlWithPassword(streamer, password string) (record.StreamLookupResult, error) {
	pageInfo := fetchStreamInfo(streamer)
	effectiveInfo := pageInfo

	u, _ := url.Parse(apiEndpoint)
	q := u.Query()
	q.Set("target", streamer)
	q.Set("mode", "client")
	u.RawQuery = q.Encode()

	request, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Referer", fmt.Sprint(baseDomain, "/", streamer))
	ApplyAuthToRequest(request)
	response, err := httpClient.Do(request)
	if err != nil {
		result := buildLookupResultFromPageInfo(effectiveInfo)
		return result, fmt.Errorf("requesting stream info failed: %w", err)
	}
	defer response.Body.Close()

	responseData := map[string]interface{}{}
	if err = json.NewDecoder(response.Body).Decode(&responseData); err != nil {
		result := buildLookupResultFromPageInfo(effectiveInfo)
		return result, err
	}
	jq := jsonq.NewQuery(responseData)

	if err = checkStreamOnline(jq); err != nil {
		result := buildLookupResultFromPageInfo(effectiveInfo)
		return result, err
	}
	if movieID, err := extractMovieID(jq); err == nil {
		if movieInfo, movieErr := fetchMovieInfo(streamer, movieID); movieErr == nil {
			effectiveInfo = mergePageInfo(effectiveInfo, movieInfo)
		} else {
			log.Printf("Failed fetching movie page info for streamer [%s] movie [%s]: %v\n", streamer, movieID, movieErr)
		}
	}
	result := buildLookupResultFromPageInfo(effectiveInfo)
	if movieID, err := extractMovieID(jq); err == nil {
		result.MovieID = strings.TrimSpace(movieID)
	}
	if detectMembersOnlyMovieFallback(effectiveInfo, result.MovieID) {
		result.Title = ""
		result.MemberOnly = true
		return result, wrapStreamLookupError(ErrMemberOnlyLive, result)
	}
	if !result.MemberOnly {
		result.MemberOnly = detectMembershipOnlyWithAuthAccess(streamer)
	}
	if effectiveInfo.memberOnly {
		return result, wrapStreamLookupError(ErrMemberOnlyLive, result)
	}
	if effectiveInfo.passwordRequired && strings.TrimSpace(password) == "" {
		return result, ErrPasswordRequired
	}

	// Try to get URL directly
	if streamURL, err := getDirectStreamURL(jq); err == nil {
		result.StreamURL = appendPasswordToken(streamURL, password)
		return result, nil
	}

	log.Printf("Direct Stream URL for streamer [%s] not available in the API response; fallback to default URL\n", streamer)
	fallbackURL, fallbackErr := fallbackStreamURL(jq)
	result.StreamURL = appendPasswordToken(fallbackURL, password)
	return result, fallbackErr
}

func buildLookupResultFromPageInfo(info streamPageInfo) record.StreamLookupResult {
	return record.StreamLookupResult{
		StreamerName: info.streamerName,
		Title:        info.title,
		AvatarURL:    info.avatarURL,
		CoverURL:     info.coverURL,
		MemberOnly:   info.memberOnly,
	}
}

func mergePageInfo(base, override streamPageInfo) streamPageInfo {
	merged := base
	if override.streamerName != "" {
		merged.streamerName = override.streamerName
	}
	if override.title != "" {
		merged.title = override.title
	}
	if override.avatarURL != "" {
		merged.avatarURL = override.avatarURL
	}
	if override.coverURL != "" {
		merged.coverURL = override.coverURL
	}
	// movie 页面比频道首页更接近实际直播，因此锁定类型优先信 movie 页面。
	merged.passwordRequired = override.passwordRequired
	merged.memberOnly = override.memberOnly
	return merged
}

func checkStreamOnline(jq *jsonq.JsonQuery) error {
	isLive, err := jq.Bool("movie", "live")
	if err != nil {
		return fmt.Errorf("error checking stream online status: %w", err)
	} else if !isLive {
		return ErrStreamOffline
	}
	return nil
}

func getDirectStreamURL(jq *jsonq.JsonQuery) (string, error) {
	// Try to get URL directly
	if streamURL, err := jq.String("llfmp4", "streams", "main"); err == nil {
		return streamURL, nil
	}
	if streamURL, err := jq.String("llfmp4", "streams", "mobilesource"); err == nil {
		return streamURL, nil
	}
	if streamURL, err := jq.String("llfmp4", "streams", "base"); err == nil {
		return streamURL, nil
	}

	return "", fmt.Errorf("direct stream URL not available")
}

func extractMovieID(jq *jsonq.JsonQuery) (string, error) {
	if movieID, err := jq.String("movie", "id"); err == nil {
		return strings.TrimSpace(movieID), nil
	}

	movieID, err := jq.Int("movie", "id")
	if err != nil {
		return "", fmt.Errorf("failed parsing movie ID: %w", err)
	}
	return fmt.Sprintf("%d", movieID), nil
}

func fallbackStreamURL(jq *jsonq.JsonQuery) (string, error) {
	mode := "base" // default mode
	if isSource, err := jq.Bool("fmp4", "source"); err == nil && isSource {
		mode = "main"
	} else if isMobile, err := jq.Bool("fmp4", "mobilesource"); err == nil && isMobile {
		mode = "mobilesource"
	}

	protocol, err := jq.String("fmp4", "proto")
	if err != nil {
		return "", fmt.Errorf("failed parsing stream protocol: %w", err)
	}

	host, err := jq.String("fmp4", "host")
	if err != nil {
		return "", fmt.Errorf("failed parsing stream host: %w", err)
	}

	movieID, err := extractMovieID(jq)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s/ws.app/stream/%s/fmp4/bd/1/1500?mode=%s", protocol, host, movieID, mode), nil
}

func sanitizeFilename(name string) string {
	return filenameSanitizer.Replace(name)
}

func fetchStreamInfo(streamer string) streamPageInfo {
	info, _, err := fetchStreamInfoWithStatus(streamer)
	if err != nil {
		return streamPageInfo{streamerName: streamer}
	}
	return info
}

func fetchStreamInfoWithStatus(streamer string) (streamPageInfo, int, error) {
	return fetchStreamInfoWithStatusUsingAuth(streamer, true)
}

func fetchStreamInfoWithStatusWithoutAuth(streamer string) (streamPageInfo, int, error) {
	return fetchStreamInfoWithStatusUsingAuth(streamer, false)
}

func fetchStreamInfoWithStatusUsingAuth(streamer string, includeAuth bool) (streamPageInfo, int, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprint(baseDomain, "/", streamer), nil)
	if err != nil {
		return streamPageInfo{streamerName: streamer}, 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	if includeAuth {
		ApplyAuthToRequest(req)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return streamPageInfo{streamerName: streamer}, 0, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return streamPageInfo{streamerName: streamer}, resp.StatusCode, err
	}

	info := parseStreamPageInfo(streamer, string(bodyBytes))
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		enrichStreamInfoFromUserAPI(streamer, &info)
	}
	return info, resp.StatusCode, nil
}

func parseStreamPageInfo(streamer, bodyStr string) streamPageInfo {
	info := streamPageInfo{
		streamerName: streamer,
		title:        "",
	}
	info.passwordRequired = requiresStreamPassword(bodyStr)
	info.memberOnly = requiresMembershipAccess(bodyStr)
	rawTitle := extractPageTitle(bodyStr)
	info.streamerName, info.title = extractPrimaryStreamerAndTitle(streamer, rawTitle, bodyStr)
	info.title = selectPreferredStreamTitle(bodyStr, rawTitle, info.streamerName, streamer, info.title)

	// TwitCasting 页面里同时有直播封面和主播头像。
	// 这里把两者拆开保存，避免再把封面误当成主播头像。
	info.coverURL = extractCoverURL(bodyStr)
	info.avatarURL = extractAvatarURL(bodyStr)
	info.displayedMovieID = extractDisplayedMovieID(bodyStr)
	info.membershipPage = hasMembershipPageHints(bodyStr)
	info.streamerName = sanitizeFilename(info.streamerName)
	info.title = sanitizeFilename(info.title)
	return info
}

func extractPageTitle(body string) string {
	match := titleTagRegex.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func extractPrimaryStreamerAndTitle(streamer, rawTitle, body string) (string, string) {
	streamerName := streamer
	title := ""

	if rawTitle != "" {
		if parsedTitle, parsedStreamer := parseTitleTag(streamer, rawTitle); parsedStreamer != "" {
			streamerName = parsedStreamer
			title = parsedTitle
		}
	}

	if streamerName == streamer {
		authorMatch := authorMetaRegex.FindStringSubmatch(body)
		if len(authorMatch) > 1 {
			candidate := strings.TrimSpace(authorMatch[1])
			if candidate != "" && !strings.EqualFold(candidate, "twitcasting") {
				streamerName = candidate
			}
		}
	}
	if streamerName == streamer {
		streamerName = extractBroadcasterName(body, streamer)
	}
	return streamerName, title
}

func selectPreferredStreamTitle(body, rawTitle, streamerName, streamer, currentTitle string) string {
	title := strings.TrimSpace(currentTitle)
	if title == "" {
		title = extractLiveHeadingTitle(body)
	}
	if title == "" {
		title = extractFallbackTwitterTitle(body, streamerName, streamer)
	}
	if title == "" {
		title = extractTitleFromPageTitle(rawTitle, streamerName, streamer)
	}
	if extended := extractExtendedTwitterDescriptionTitle(body, title, streamerName, streamer); extended != "" {
		return extended
	}
	return title
}

func extractLiveHeadingTitle(body string) string {
	match := liveTitleHeadingRegex.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	return cleanHTMLText(match[1])
}

func extractBroadcasterName(body, fallback string) string {
	match := broadcasterNameDataRegex.FindStringSubmatch(body)
	if len(match) < 2 {
		return fallback
	}
	candidate := cleanHTMLText(match[1])
	if candidate == "" {
		return fallback
	}
	return candidate
}

func cleanHTMLText(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	text := html.UnescapeString(raw)
	text = htmlTagRegex.ReplaceAllString(text, " ")
	return strings.Join(strings.Fields(text), " ")
}

func extractFallbackTwitterTitle(body, streamerName, streamer string) string {
	match := twitterTitleMetaRegex.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	candidate := strings.TrimSpace(match[1])
	if !isUsableTitleCandidate(candidate, streamerName, streamer) {
		return ""
	}
	return candidate
}

func extractTitleFromPageTitle(rawTitle, streamerName, streamer string) string {
	if rawTitle == "" || streamerName == streamer {
		return ""
	}
	marker := " - " + streamerName
	if !strings.Contains(rawTitle, marker) {
		return ""
	}
	parts := strings.SplitN(rawTitle, marker, 2)
	candidate := strings.TrimSpace(parts[0])
	if !isUsableTitleCandidate(candidate, streamerName, streamer) {
		return ""
	}
	return candidate
}

func isUsableTitleCandidate(candidate, streamerName, streamer string) bool {
	candidate = strings.TrimSpace(candidate)
	return candidate != "" &&
		candidate != streamerName &&
		!strings.Contains(candidate, "@"+streamer)
}

func extractExtendedTwitterDescriptionTitle(body, currentTitle, streamerName, streamer string) string {
	baseTitle := cleanHTMLText(currentTitle)
	if !isUsableTitleCandidate(baseTitle, streamerName, streamer) {
		return ""
	}

	description := extractMetaContentByPatterns(body, twitterDescriptionMetaRegexes)
	description = cleanHTMLText(description)
	if !isUsableTitleCandidate(description, streamerName, streamer) {
		return ""
	}
	if description == baseTitle {
		return ""
	}
	if !strings.HasPrefix(description, baseTitle) {
		return ""
	}
	return description
}

func extractDisplayedMovieID(body string) string {
	match := pageMovieIDDataRegex.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func hasMembershipPageHints(body string) bool {
	lower := strings.ToLower(body)
	return strings.Contains(lower, "membershipjoindetail.php?u=") ||
		strings.Contains(lower, "tw-membership-button") ||
		strings.Contains(lower, "tw-unit-own-member-icon")
}

func detectMembersOnlyMovieFallback(info streamPageInfo, currentMovieID string) bool {
	displayedMovieID := strings.TrimSpace(info.displayedMovieID)
	trimmedCurrentMovieID := strings.TrimSpace(currentMovieID)
	if displayedMovieID == "" || trimmedCurrentMovieID == "" {
		return false
	}
	if displayedMovieID == trimmedCurrentMovieID {
		return false
	}
	return info.membershipPage
}

func detectMembershipOnlyWithAuthAccess(streamer string) bool {
	if strings.TrimSpace(CurrentCookieHeader()) == "" {
		return false
	}

	info, statusCode, err := fetchStreamInfoWithStatusWithoutAuth(streamer)
	if err != nil || statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return false
	}
	return info.memberOnly
}

func parseTitleTag(streamer, rawTitle string) (string, string) {
	marker := "(@" + streamer + ")"
	idx := strings.Index(rawTitle, marker)
	if idx < 0 {
		return "", ""
	}

	prefix := strings.TrimSpace(rawTitle[:idx])
	if prefix == "" {
		return "", ""
	}
	if separator := strings.Index(prefix, " - "); separator >= 0 {
		return strings.TrimSpace(prefix[:separator]), strings.TrimSpace(prefix[separator+3:])
	}
	return "", prefix
}

func extractAvatarURL(body string) string {
	for _, pattern := range avatarURLRegexes {
		match := pattern.FindStringSubmatch(body)
		if len(match) > 1 {
			return normalizeImageURL(match[1])
		}
	}
	return ""
}

func extractCoverURL(body string) string {
	for _, target := range []struct {
		attr string
		name string
	}{
		{attr: "property", name: "og:image"},
		{attr: "name", name: "twitter:image"},
	} {
		if candidate := extractMetaContent(body, target.attr, target.name); candidate != "" {
			return normalizeImageURL(candidate)
		}
	}
	return ""
}

func extractMetaContent(body, attr, name string) string {
	var patterns []*regexp.Regexp
	switch {
	case attr == "property" && name == "og:image":
		patterns = ogImageMetaRegexes
	case attr == "name" && name == "twitter:image":
		patterns = twitterImageMetaRegexes
	default:
		return ""
	}

	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(body)
		if len(match) > 1 {
			return strings.TrimSpace(html.UnescapeString(match[1]))
		}
	}
	return ""
}

func extractMetaContentByPatterns(body string, patterns []*regexp.Regexp) string {
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(body)
		if len(match) > 1 {
			return strings.TrimSpace(html.UnescapeString(match[1]))
		}
	}
	return ""
}

func compileMetaContentRegexes(attr, name string) []*regexp.Regexp {
	quotedName := regexp.QuoteMeta(name)
	return []*regexp.Regexp{
		regexp.MustCompile(fmt.Sprintf(`(?is)<meta[^>]+\b%s=["']%s["'][^>]+\bcontent=["']([^"'<>]+)["']`, attr, quotedName)),
		regexp.MustCompile(fmt.Sprintf(`(?is)<meta[^>]+\bcontent=["']([^"'<>]+)["'][^>]+\b%s=["']%s["']`, attr, quotedName)),
	}
}

func normalizeImageURL(raw string) string {
	candidate := strings.TrimSpace(html.UnescapeString(raw))
	if candidate == "" {
		return ""
	}

	lower := strings.ToLower(candidate)
	switch {
	case strings.HasPrefix(candidate, "//"):
		candidate = "https:" + candidate
	case strings.HasPrefix(lower, "http://"):
		candidate = "https://" + candidate[len("http://"):]
	}

	return upgradeImageURL(candidate)
}

func upgradeImageURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	if strings.Contains(parsed.Path, "/image3s/") {
		parsed.Path = strings.Replace(parsed.Path, "/image3s/", "/image3/", 1)
	}
	parsed.Path = twitterProfileSizeSuffixRegex.ReplaceAllString(parsed.Path, `$2`)
	parsed.Path = genericSmallImageSuffixRegex.ReplaceAllString(parsed.Path, `$1`)
	return parsed.String()
}

// 锁页会显示专门的空状态和 password 表单；先在这里拦下，避免无密码时反复尝试启动录影。
func requiresStreamPassword(body string) bool {
	lower := strings.ToLower(body)
	return strings.Contains(lower, "tw-player-page-lock-empty-state") &&
		strings.Contains(lower, `name="password"`)
}

// 会员限定页和密码锁页一样，都会出现锁定空状态；这里用多个文案与 join membership 链接做 best-effort 识别。
func requiresMembershipAccess(body string) bool {
	lower := strings.ToLower(body)
	if !strings.Contains(lower, "tw-player-page-lock-empty-state") {
		return false
	}

	for _, marker := range membershipMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func appendPasswordToken(streamURL, password string) string {
	if strings.TrimSpace(streamURL) == "" || strings.TrimSpace(password) == "" {
		return streamURL
	}

	parsed, err := url.Parse(streamURL)
	if err != nil {
		return streamURL
	}

	query := parsed.Query()
	query.Set("word", md5Hex(strings.TrimSpace(password)))
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func md5Hex(text string) string {
	sum := md5.Sum([]byte(text))
	return hex.EncodeToString(sum[:])
}
