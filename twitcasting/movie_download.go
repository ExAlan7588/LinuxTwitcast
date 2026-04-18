package twitcasting

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrMovieMembersOnly      = errors.New("movie archive is members-only; verify that the uploaded cookie still has membership access")
	ErrMoviePasswordRequired = errors.New("movie archive requires a password")
	ErrMoviePlaylistMissing  = errors.New("movie archive playlist was not found on the page")

	moviePlaylistAttrRegexes = []*regexp.Regexp{
		regexp.MustCompile(`(?is)data-movie-playlist='([^']+)'`),
		regexp.MustCompile(`(?is)data-movie-playlist="([^"]+)"`),
	}
	movieBitrateSelectedAttrRegexes = []*regexp.Regexp{
		regexp.MustCompile(`(?is)data-adaptive-bitrate-selected='([^']+)'`),
		regexp.MustCompile(`(?is)data-adaptive-bitrate-selected="([^"]+)"`),
	}
	movieCSSessionIDRegex  = regexp.MustCompile(`(?is)name=["']cs_session_id["'][^>]*value=["']([^"']+)["']`)
	movieCreatedUnixRegex  = regexp.MustCompile(`(?is)id=["']created_unix_time["'][^>]*value=["'](\d+)["']`)
	movieTimeDatetimeRegex = regexp.MustCompile(`(?is)<time[^>]+\bdatetime=["']([^"']+)["']`)
	lookPathFFmpeg         = exec.LookPath
	runMovieDownloadFFmpeg = func(args ...string) ([]byte, error) {
		return exec.Command("ffmpeg", args...).CombinedOutput()
	}
)

type MovieDownloadInfo struct {
	ScreenID     string
	MovieID      string
	MovieURL     string
	StreamerName string
	Title        string
	AvatarURL    string
	CoverURL     string
	StartedAt    time.Time
	PlaylistURLs []string
	CookieHeader string
}

type moviePlaylistEntry struct {
	Source struct {
		URL string `json:"url"`
	} `json:"source"`
}

// 归档 movie 的下载要先拿到页面里的 data-movie-playlist。
// 这一步同时复用登录 cookie，并在需要时用密码表单解锁页面。
func PrepareMovieDownload(screenID, movieID, password string) (MovieDownloadInfo, error) {
	trimmedScreenID := strings.TrimSpace(screenID)
	trimmedMovieID := strings.TrimSpace(movieID)
	if trimmedScreenID == "" || trimmedMovieID == "" {
		return MovieDownloadInfo{}, errors.New("screen-id and movie-id are required")
	}

	movieURL := fmt.Sprintf("%s/%s/movie/%s", baseDomain, trimmedScreenID, trimmedMovieID)
	client, jar, err := newMovieDownloadHTTPClient()
	if err != nil {
		return MovieDownloadInfo{}, err
	}

	body, cookieHeader, err := fetchMovieBodyForDownload(client, jar, movieURL, strings.TrimSpace(password))
	info := buildMovieDownloadInfo(trimmedScreenID, trimmedMovieID, movieURL, body, cookieHeader)
	if err != nil {
		return info, err
	}

	playlistURLs, playlistErr := extractMoviePlaylistURLs(body)
	if playlistErr != nil {
		return info, playlistErr
	}

	info.PlaylistURLs = playlistURLs
	return info, nil
}

func DownloadMovieArchive(info MovieDownloadInfo, folder string) ([]string, error) {
	if _, err := lookPathFFmpeg("ffmpeg"); err != nil {
		return nil, errors.New("ffmpeg is not available in PATH")
	}

	targetFolder := strings.TrimSpace(folder)
	if targetFolder == "" {
		targetFolder = filepath.Join("Recordings", strings.TrimSpace(info.ScreenID))
	}
	if err := os.MkdirAll(targetFolder, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create movie download folder %s: %w", targetFolder, err)
	}

	outputs := make([]string, 0, len(info.PlaylistURLs))
	for index, playlistURL := range info.PlaylistURLs {
		outputPath := nextAvailableMovieOutputPath(targetFolder, buildMovieDownloadFilename(info, index, len(info.PlaylistURLs)))
		args := buildMovieDownloadArgs(info.MovieURL, info.CookieHeader, playlistURL, outputPath)
		log.Printf("[Manual] Downloading archived movie [%s] part %d/%d to %s\n", info.MovieID, index+1, len(info.PlaylistURLs), outputPath)
		out, err := runMovieDownloadFFmpeg(args...)
		if err != nil {
			_ = os.Remove(outputPath)
			return outputs, fmt.Errorf("ffmpeg failed downloading movie %s part %d/%d: %v | %s", info.MovieID, index+1, len(info.PlaylistURLs), err, strings.TrimSpace(string(out)))
		}
		outputs = append(outputs, outputPath)
	}

	return outputs, nil
}

func PlannedMovieDownloadOutputs(info MovieDownloadInfo, folder string) []string {
	targetFolder := strings.TrimSpace(folder)
	if targetFolder == "" {
		targetFolder = filepath.Join("Recordings", strings.TrimSpace(info.ScreenID))
	}

	outputs := make([]string, 0, len(info.PlaylistURLs))
	for index := range info.PlaylistURLs {
		outputs = append(outputs, filepath.Join(targetFolder, buildMovieDownloadFilename(info, index, len(info.PlaylistURLs))))
	}
	return outputs
}

func buildMovieDownloadInfo(screenID, movieID, movieURL, body, cookieHeader string) MovieDownloadInfo {
	pageInfo := parseStreamPageInfo(screenID, body)
	return MovieDownloadInfo{
		ScreenID:     screenID,
		MovieID:      movieID,
		MovieURL:     movieURL,
		StreamerName: strings.TrimSpace(pageInfo.streamerName),
		Title:        strings.TrimSpace(pageInfo.title),
		AvatarURL:    strings.TrimSpace(pageInfo.avatarURL),
		CoverURL:     strings.TrimSpace(pageInfo.coverURL),
		StartedAt:    extractMovieStartedAt(body),
		CookieHeader: strings.TrimSpace(cookieHeader),
	}
}

func newMovieDownloadHTTPClient() (*http.Client, http.CookieJar, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, nil, err
	}

	client := &http.Client{
		Timeout: requestTimeout,
		Jar:     jar,
	}
	if httpClient != nil {
		client.Timeout = httpClient.Timeout
		client.Transport = httpClient.Transport
		client.CheckRedirect = httpClient.CheckRedirect
	}

	return client, jar, nil
}

func fetchMovieBodyForDownload(client *http.Client, jar http.CookieJar, movieURL, password string) (string, string, error) {
	body, err := requestMoviePage(client, movieURL, "", "")
	if err != nil {
		return "", "", err
	}
	if _, err := extractMoviePlaylistURLs(body); err == nil {
		return body, buildMovieCookieHeader(jar, movieURL), nil
	}
	if requiresMembershipAccess(body) {
		return body, buildMovieCookieHeader(jar, movieURL), ErrMovieMembersOnly
	}
	if !requiresStreamPassword(body) {
		return body, buildMovieCookieHeader(jar, movieURL), ErrMoviePlaylistMissing
	}
	if strings.TrimSpace(password) == "" {
		return body, buildMovieCookieHeader(jar, movieURL), ErrMoviePasswordRequired
	}

	csSessionID := extractMovieCSSessionID(body)
	if csSessionID == "" {
		return body, buildMovieCookieHeader(jar, movieURL), errors.New("failed to locate movie password form token")
	}

	form := url.Values{}
	form.Set("password", strings.TrimSpace(password))
	form.Set("cs_session_id", csSessionID)

	unlockedBody, err := requestMoviePage(client, movieURL, movieURL, form.Encode())
	if err != nil {
		return body, buildMovieCookieHeader(jar, movieURL), err
	}
	if _, err := extractMoviePlaylistURLs(unlockedBody); err == nil {
		return unlockedBody, buildMovieCookieHeader(jar, movieURL), nil
	}
	if requiresMembershipAccess(unlockedBody) {
		return unlockedBody, buildMovieCookieHeader(jar, movieURL), ErrMovieMembersOnly
	}
	if requiresStreamPassword(unlockedBody) {
		return unlockedBody, buildMovieCookieHeader(jar, movieURL), ErrMoviePasswordRequired
	}
	return unlockedBody, buildMovieCookieHeader(jar, movieURL), ErrMoviePlaylistMissing
}

func requestMoviePage(client *http.Client, movieURL, referer, formBody string) (string, error) {
	var (
		req *http.Request
		err error
	)

	if formBody == "" {
		req, err = http.NewRequest(http.MethodGet, movieURL, nil)
	} else {
		req, err = http.NewRequest(http.MethodPost, movieURL, strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", userAgent)
	if strings.TrimSpace(referer) != "" {
		req.Header.Set("Referer", referer)
	}
	ApplyAuthToRequest(req)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("unexpected movie page status: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bodyBytes), nil
}

func extractMoviePlaylistURLs(body string) ([]string, error) {
	decoded := ""
	for _, pattern := range moviePlaylistAttrRegexes {
		match := pattern.FindStringSubmatch(body)
		if len(match) < 2 {
			continue
		}
		decoded = html.UnescapeString(strings.TrimSpace(match[1]))
		if decoded != "" {
			break
		}
	}
	if decoded == "" {
		return nil, ErrMoviePlaylistMissing
	}

	playlistMap := make(map[string][]moviePlaylistEntry)
	if err := json.Unmarshal([]byte(decoded), &playlistMap); err != nil {
		return nil, fmt.Errorf("failed parsing movie playlist JSON: %w", err)
	}

	selectedKey := extractMovieSelectedBitrate(body)
	entries := selectMoviePlaylistEntries(playlistMap, selectedKey)
	if len(entries) == 0 {
		return nil, ErrMoviePlaylistMissing
	}

	urls := make([]string, 0, len(entries))
	for _, entry := range entries {
		candidate := strings.TrimSpace(entry.Source.URL)
		if candidate == "" {
			continue
		}
		urls = append(urls, candidate)
	}
	if len(urls) == 0 {
		return nil, ErrMoviePlaylistMissing
	}
	return urls, nil
}

func extractMovieSelectedBitrate(body string) string {
	for _, pattern := range movieBitrateSelectedAttrRegexes {
		match := pattern.FindStringSubmatch(body)
		if len(match) < 2 {
			continue
		}
		return strings.TrimSpace(match[1])
	}
	return ""
}

func selectMoviePlaylistEntries(playlistMap map[string][]moviePlaylistEntry, selectedKey string) []moviePlaylistEntry {
	if entries := playlistMap[strings.TrimSpace(selectedKey)]; len(entries) > 0 {
		return entries
	}

	keys := make([]string, 0, len(playlistMap))
	for key, entries := range playlistMap {
		if len(entries) == 0 {
			continue
		}
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left, leftErr := strconv.Atoi(keys[i])
		right, rightErr := strconv.Atoi(keys[j])
		if leftErr == nil && rightErr == nil {
			return left > right
		}
		return keys[i] > keys[j]
	})
	if len(keys) == 0 {
		return nil
	}
	return playlistMap[keys[0]]
}

func extractMovieCSSessionID(body string) string {
	match := movieCSSessionIDRegex.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func extractMovieStartedAt(body string) time.Time {
	if match := movieCreatedUnixRegex.FindStringSubmatch(body); len(match) > 1 {
		if unixValue, err := strconv.ParseInt(strings.TrimSpace(match[1]), 10, 64); err == nil && unixValue > 0 {
			return time.Unix(unixValue, 0)
		}
	}

	if match := movieTimeDatetimeRegex.FindStringSubmatch(body); len(match) > 1 {
		candidate := strings.TrimSpace(match[1])
		for _, layout := range []string{time.RFC1123Z, time.RFC1123, "2006/01/02 15:04:05"} {
			if parsed, err := time.Parse(layout, candidate); err == nil {
				return parsed
			}
		}
	}

	return time.Time{}
}

func buildMovieCookieHeader(jar http.CookieJar, movieURL string) string {
	pairs := make([]string, 0, 4)
	seen := make(map[string]int, 4)

	appendHeader := func(header string) {
		normalized, ok := parseCookieHeader(header)
		if !ok {
			return
		}
		for _, part := range strings.Split(normalized, ";") {
			token := strings.TrimSpace(part)
			if token == "" {
				continue
			}
			name, value, ok := strings.Cut(token, "=")
			if !ok {
				continue
			}
			pair := fmt.Sprintf("%s=%s", strings.TrimSpace(name), strings.TrimSpace(value))
			if idx, exists := seen[name]; exists {
				pairs[idx] = pair
				continue
			}
			seen[name] = len(pairs)
			pairs = append(pairs, pair)
		}
	}

	appendHeader(CurrentCookieHeader())
	if jar != nil {
		if parsedURL, err := url.Parse(movieURL); err == nil {
			for _, cookie := range jar.Cookies(parsedURL) {
				if strings.TrimSpace(cookie.Name) == "" {
					continue
				}
				pair := fmt.Sprintf("%s=%s", cookie.Name, cookie.Value)
				if idx, exists := seen[cookie.Name]; exists {
					pairs[idx] = pair
					continue
				}
				seen[cookie.Name] = len(pairs)
				pairs = append(pairs, pair)
			}
		}
	}

	return strings.Join(pairs, "; ")
}

func buildMovieDownloadFilename(info MovieDownloadInfo, index, total int) string {
	dateLabel := time.Now().Format("2006-01-02")
	if !info.StartedAt.IsZero() {
		dateLabel = info.StartedAt.Local().Format("2006-01-02")
	}

	streamerName := strings.TrimSpace(info.StreamerName)
	if streamerName == "" {
		streamerName = strings.TrimSpace(info.ScreenID)
	}

	title := strings.TrimSpace(info.Title)
	if title == "" {
		title = strings.TrimSpace(info.MovieID)
	}

	filename := fmt.Sprintf("[%s][%s]%s", streamerName, dateLabel, title)
	if total > 1 {
		filename += fmt.Sprintf(".part%d", index+1)
	}
	return filename + ".mp4"
}

func nextAvailableMovieOutputPath(folder, filename string) string {
	candidate := filepath.Join(folder, filename)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}

	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	for index := 2; ; index++ {
		next := filepath.Join(folder, fmt.Sprintf("%s (%d)%s", base, index, ext))
		if _, err := os.Stat(next); os.IsNotExist(err) {
			return next
		}
	}
}

func buildMovieDownloadArgs(movieURL, cookieHeader, playlistURL, outputPath string) []string {
	args := []string{
		"-y",
		"-protocol_whitelist", "file,http,https,tcp,tls,crypto",
		"-user_agent", userAgent,
	}

	headers := []string{
		fmt.Sprintf("Origin: %s", baseDomain),
		fmt.Sprintf("Referer: %s", movieURL),
	}
	if strings.TrimSpace(cookieHeader) != "" {
		headers = append(headers, fmt.Sprintf("Cookie: %s", cookieHeader))
	}
	args = append(args, "-headers", strings.Join(headers, "\r\n")+"\r\n")
	// archived movie 是完整 HLS 播放列表，不像 live 直錄那樣容易中斷；
	// 這裡直接封裝成 mp4，並補 faststart 讓一般播放器與網頁播放更友善。
	args = append(args, "-i", playlistURL, "-c", "copy", "-movflags", "+faststart", outputPath)
	return args
}
