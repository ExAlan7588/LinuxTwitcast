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

type streamPageInfo struct {
	streamerName     string
	title            string
	avatarURL        string
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
	result := record.StreamLookupResult{
		StreamerName: pageInfo.streamerName,
		Title:        pageInfo.title,
		AvatarURL:    pageInfo.avatarURL,
	}
	if pageInfo.passwordRequired && strings.TrimSpace(password) == "" {
		return result, ErrPasswordRequired
	}

	u, _ := url.Parse(apiEndpoint)
	q := u.Query()
	q.Set("target", streamer)
	q.Set("mode", "client")
	u.RawQuery = q.Encode()

	request, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Referer", fmt.Sprint(baseDomain, "/", streamer))
	response, err := httpClient.Do(request)
	if err != nil {
		return result, fmt.Errorf("requesting stream info failed: %w", err)
	}
	defer response.Body.Close()

	responseData := map[string]interface{}{}
	if err = json.NewDecoder(response.Body).Decode(&responseData); err != nil {
		return result, err
	}
	jq := jsonq.NewQuery(responseData)

	if err = checkStreamOnline(jq); err != nil {
		return result, err
	}
	if pageInfo.memberOnly {
		return result, wrapStreamLookupError(ErrMemberOnlyLive, result)
	}

	// Try to get URL directly
	if streamUrl, err := getDirectStreamUrl(jq); err == nil {
		result.StreamURL = appendPasswordToken(streamUrl, password)
		return result, nil
	}

	log.Printf("Direct Stream URL for streamer [%s] not available in the API response; fallback to default URL\n", streamer)
	fallbackUrl, fallbackErr := fallbackStreamUrl(jq)
	result.StreamURL = appendPasswordToken(fallbackUrl, password)
	return result, fallbackErr
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

func getDirectStreamUrl(jq *jsonq.JsonQuery) (string, error) {
	// Try to get URL directly
	if streamUrl, err := jq.String("llfmp4", "streams", "main"); err == nil {
		return streamUrl, nil
	}
	if streamUrl, err := jq.String("llfmp4", "streams", "mobilesource"); err == nil {
		return streamUrl, nil
	}
	if streamUrl, err := jq.String("llfmp4", "streams", "base"); err == nil {
		return streamUrl, nil
	}

	return "", fmt.Errorf("direct stream URL not available")
}

func fallbackStreamUrl(jq *jsonq.JsonQuery) (string, error) {
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

	movieId, err := jq.String("movie", "id")
	if err != nil {
		return "", fmt.Errorf("failed parsing movie ID: %w", err)
	}

	return fmt.Sprintf("%s:%s/ws.app/stream/%s/fmp4/bd/1/1500?mode=%s", protocol, host, movieId, mode), nil
}

func sanitizeFilename(name string) string {
	replacements := map[string]string{
		"\\": "＼",
		"/":  "／",
		":":  "：",
		"*":  "＊",
		"?":  "？",
		"\"": "”",
		"<":  "＜",
		">":  "＞",
		"|":  "｜",
	}
	for old, new := range replacements {
		name = strings.ReplaceAll(name, old, new)
	}
	return name
}

func fetchStreamInfo(streamer string) streamPageInfo {
	info, _, err := fetchStreamInfoWithStatus(streamer)
	if err != nil {
		return streamPageInfo{streamerName: streamer}
	}
	return info
}

func fetchStreamInfoWithStatus(streamer string) (streamPageInfo, int, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprint(baseDomain, "/", streamer), nil)
	if err != nil {
		return streamPageInfo{streamerName: streamer}, 0, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return streamPageInfo{streamerName: streamer}, 0, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return streamPageInfo{streamerName: streamer}, resp.StatusCode, err
	}
	return parseStreamPageInfo(streamer, string(bodyBytes)), resp.StatusCode, nil
}

func parseStreamPageInfo(streamer, bodyStr string) streamPageInfo {
	info := streamPageInfo{
		streamerName: streamer,
		title:        "",
	}
	info.passwordRequired = requiresStreamPassword(bodyStr)
	info.memberOnly = requiresMembershipAccess(bodyStr)

	// --- Extract streamer display name ---
	// TwitCasting <title> when live: "STREAM_TITLE - NAME (@screen-id) - Twitcast"
	// TwitCasting <title> when offline: "NAME (@screen-id) 's Live - Twitcast"
	titleTagRegex := regexp.MustCompile(`<title>(.*?)</title>`)
	titleMatches := titleTagRegex.FindStringSubmatch(bodyStr)
	if len(titleMatches) > 1 {
		rawTitle := titleMatches[1]
		liveTitleRegex := regexp.MustCompile(`(?is)^(.*?)\s+-\s+(.+?)\s*\(@` + regexp.QuoteMeta(streamer) + `\)`)
		if liveMatch := liveTitleRegex.FindStringSubmatch(rawTitle); len(liveMatch) > 2 {
			info.title = strings.TrimSpace(liveMatch[1])
			info.streamerName = strings.TrimSpace(liveMatch[2])
		} else {
			offlineNameRegex := regexp.MustCompile(`(?is)^(.+?)\s*\(@` + regexp.QuoteMeta(streamer) + `\)`)
			if nameMatch := offlineNameRegex.FindStringSubmatch(rawTitle); len(nameMatch) > 1 {
				info.streamerName = strings.TrimSpace(nameMatch[1])
			}
		}
	}

	// Fallback: try <meta name="author"> for streamer name
	if info.streamerName == streamer {
		authorRegex := regexp.MustCompile(`<meta\s+name="author"\s+content="([^"]+)"`)
		authorMatch := authorRegex.FindStringSubmatch(bodyStr)
		if len(authorMatch) > 1 {
			candidate := strings.TrimSpace(authorMatch[1])
			if candidate != "" && !strings.EqualFold(candidate, "twitcasting") {
				info.streamerName = candidate
			}
		}
	}

	// --- Extract stream title ---
	// Only use twitter:title as a fallback.
	// If we already got a title earlier, do not overwrite it.
	twitterTitleRegex := regexp.MustCompile(`<meta\s+name="twitter:title"\s+content="([^"]*)"`)
	twitterTitleMatch := twitterTitleRegex.FindStringSubmatch(bodyStr)
	if len(twitterTitleMatch) > 1 {
		candidate := strings.TrimSpace(twitterTitleMatch[1])
		if info.title == "" &&
			candidate != "" &&
			candidate != info.streamerName &&
			!strings.Contains(candidate, "@"+streamer) {
			info.title = candidate
		}
	}

	// Fallback: try to extract stream title from <title> tag
	// Pattern: "STREAM_TITLE - NAME (@screen-id)"
	if info.title == "" && len(titleMatches) > 1 {
		rawTitle := titleMatches[1]
		// If the raw title contains " - NAME", the part before it is the stream title
		if info.streamerName != streamer && strings.Contains(rawTitle, " - "+info.streamerName) {
			parts := strings.SplitN(rawTitle, " - "+info.streamerName, 2)
			candidate := strings.TrimSpace(parts[0])
			// Don't treat the streamer name line itself as a title
			if candidate != "" && candidate != info.streamerName {
				info.title = candidate
			}
		}
	}

	// TwitCasting 页面里同时有直播封面和主播头像。
	// 这里先抓 broadcaster profile image，最后才 fallback 到 og:image / twitter:image。
	info.avatarURL = extractAvatarURL(bodyStr)
	info.streamerName = sanitizeFilename(info.streamerName)
	info.title = sanitizeFilename(info.title)
	return info
}

func extractAvatarURL(body string) string {
	for _, pattern := range []string{
		`(?is)\bdata-broadcaster-profile-image=["']([^"'<>]+)["']`,
		`(?is)<img[^>]+\bclass=["'][^"']*\bauthorthumbnail\b[^"']*["'][^>]+\bsrc=["']([^"'<>]+)["']`,
		`(?is)<img[^>]+\bsrc=["']([^"'<>]+)["'][^>]+\bclass=["'][^"']*\bauthorthumbnail\b[^"']*["']`,
		`(?is)<div[^>]+\bclass=["'][^"']*\btw-user-nav2-icon\b[^"']*["'][^>]*>\s*<img[^>]+\bsrc=["']([^"'<>]+)["']`,
		`(?is)<img[^>]+\bclass=["'][^"']*\btw-unit-own-member-icon\b[^"']*["'][^>]+\bsrc=["']([^"'<>]+)["']`,
	} {
		match := regexp.MustCompile(pattern).FindStringSubmatch(body)
		if len(match) > 1 {
			return normalizeImageURL(match[1])
		}
	}

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
	quotedName := regexp.QuoteMeta(name)
	patterns := []string{
		fmt.Sprintf(`(?is)<meta[^>]+\b%s=["']%s["'][^>]+\bcontent=["']([^"'<>]+)["']`, attr, quotedName),
		fmt.Sprintf(`(?is)<meta[^>]+\bcontent=["']([^"'<>]+)["'][^>]+\b%s=["']%s["']`, attr, quotedName),
	}
	for _, pattern := range patterns {
		match := regexp.MustCompile(pattern).FindStringSubmatch(body)
		if len(match) > 1 {
			return strings.TrimSpace(html.UnescapeString(match[1]))
		}
	}
	return ""
}

func normalizeImageURL(raw string) string {
	candidate := strings.TrimSpace(html.UnescapeString(raw))
	lower := strings.ToLower(candidate)
	switch {
	case strings.HasPrefix(candidate, "//"):
		return "https:" + candidate
	case strings.HasPrefix(lower, "http://"):
		return "https://" + candidate[len("http://"):]
	default:
		return candidate
	}
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

	markers := []string{
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
	for _, marker := range markers {
		if strings.Contains(lower, strings.ToLower(marker)) {
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
