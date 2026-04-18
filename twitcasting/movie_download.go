package twitcasting

import (
	"bufio"
	"bytes"
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
	ScreenID          string
	MovieID           string
	MovieURL          string
	StreamerName      string
	Title             string
	AvatarURL         string
	CoverURL          string
	StartedAt         time.Time
	PlaylistURLs      []string
	PlaylistDurations []time.Duration
	CookieHeader      string
}

type MovieDownloadProgress struct {
	PartIndex       int
	PartCount       int
	OutputPath      string
	ProgressPercent float64
}

type moviePlaylistEntry struct {
	StartTime int64 `json:"startTime"`
	Duration  int64 `json:"duration"`
	Source    struct {
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

	playlistEntries, playlistErr := extractMoviePlaylistEntries(body)
	if playlistErr != nil {
		return info, playlistErr
	}

	info.PlaylistURLs = moviePlaylistEntryURLs(playlistEntries)
	if len(info.PlaylistURLs) == 0 {
		return info, ErrMoviePlaylistMissing
	}
	info.PlaylistDurations = moviePlaylistEntryDurations(playlistEntries)
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

func DownloadMovieArchiveWithProgress(info MovieDownloadInfo, folder string, onProgress func(MovieDownloadProgress)) ([]string, error) {
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
	totalDuration := totalMovieDownloadDuration(info.PlaylistDurations)
	completedDuration := time.Duration(0)

	for index, playlistURL := range info.PlaylistURLs {
		outputPath := nextAvailableMovieOutputPath(targetFolder, buildMovieDownloadFilename(info, index, len(info.PlaylistURLs)))
		if onProgress != nil {
			onProgress(buildMovieDownloadProgress(info, index, outputPath, completedDuration, 0, totalDuration))
		}

		args := buildMovieDownloadProgressArgs(info.MovieURL, info.CookieHeader, playlistURL, outputPath)
		log.Printf("[Manual] Downloading archived movie [%s] part %d/%d to %s\n", info.MovieID, index+1, len(info.PlaylistURLs), outputPath)
		stderr, err := runMovieDownloadWithProgress(args, func(currentDuration time.Duration) {
			if onProgress == nil {
				return
			}
			onProgress(buildMovieDownloadProgress(info, index, outputPath, completedDuration, currentDuration, totalDuration))
		})
		if err != nil {
			_ = os.Remove(outputPath)
			return outputs, fmt.Errorf("ffmpeg failed downloading movie %s part %d/%d: %v | %s", info.MovieID, index+1, len(info.PlaylistURLs), err, strings.TrimSpace(string(stderr)))
		}

		if onProgress != nil {
			onProgress(buildMovieDownloadProgress(info, index, outputPath, completedDuration, moviePlaylistDurationAt(info, index), totalDuration))
		}
		completedDuration += moviePlaylistDurationAt(info, index)
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
	entries, err := extractMoviePlaylistEntries(body)
	if err != nil {
		return nil, err
	}
	urls := moviePlaylistEntryURLs(entries)
	if len(urls) == 0 {
		return nil, ErrMoviePlaylistMissing
	}
	return urls, nil
}

func extractMoviePlaylistEntries(body string) ([]moviePlaylistEntry, error) {
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

	return entries, nil
}

func moviePlaylistEntryURLs(entries []moviePlaylistEntry) []string {
	urls := make([]string, 0, len(entries))
	for _, entry := range entries {
		candidate := strings.TrimSpace(entry.Source.URL)
		if candidate == "" {
			continue
		}
		urls = append(urls, candidate)
	}
	if len(urls) == 0 {
		return nil
	}
	return urls
}

func moviePlaylistEntryDurations(entries []moviePlaylistEntry) []time.Duration {
	durations := make([]time.Duration, 0, len(entries))
	for _, entry := range entries {
		if entry.Duration <= 0 {
			durations = append(durations, 0)
			continue
		}
		durations = append(durations, time.Duration(entry.Duration)*time.Millisecond)
	}
	return durations
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

func buildMovieDownloadProgressArgs(movieURL, cookieHeader, playlistURL, outputPath string) []string {
	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-nostats",
		"-progress", "pipe:1",
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
	// 进度回调用 ffmpeg 的 progress pipe，不影响最终仍输出 mp4。
	args = append(args, "-i", playlistURL, "-c", "copy", "-movflags", "+faststart", outputPath)
	return args
}

func totalMovieDownloadDuration(durations []time.Duration) time.Duration {
	total := time.Duration(0)
	for _, duration := range durations {
		if duration > 0 {
			total += duration
		}
	}
	return total
}

func moviePlaylistDurationAt(info MovieDownloadInfo, index int) time.Duration {
	if index < 0 || index >= len(info.PlaylistDurations) {
		return 0
	}
	return info.PlaylistDurations[index]
}

func buildMovieDownloadProgress(info MovieDownloadInfo, partIndex int, outputPath string, completedBefore time.Duration, currentDuration time.Duration, totalDuration time.Duration) MovieDownloadProgress {
	partCount := len(info.PlaylistURLs)
	partDuration := moviePlaylistDurationAt(info, partIndex)
	if currentDuration < 0 {
		currentDuration = 0
	}
	if partDuration > 0 && currentDuration > partDuration {
		currentDuration = partDuration
	}

	partFraction := 0.0
	if partDuration > 0 {
		partFraction = float64(currentDuration) / float64(partDuration)
	}
	if partFraction < 0 {
		partFraction = 0
	}
	if partFraction > 1 {
		partFraction = 1
	}

	progressPercent := 0.0
	switch {
	case totalDuration > 0:
		downloaded := completedBefore + currentDuration
		if downloaded > totalDuration {
			downloaded = totalDuration
		}
		progressPercent = (float64(downloaded) / float64(totalDuration)) * 100
	case partCount > 0:
		progressPercent = ((float64(partIndex) + partFraction) / float64(partCount)) * 100
	}
	if progressPercent < 0 {
		progressPercent = 0
	}
	if progressPercent > 100 {
		progressPercent = 100
	}

	return MovieDownloadProgress{
		PartIndex:       partIndex,
		PartCount:       partCount,
		OutputPath:      outputPath,
		ProgressPercent: progressPercent,
	}
}

func runMovieDownloadWithProgress(args []string, onProgress func(time.Duration)) ([]byte, error) {
	cmd := exec.Command("ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	scanErrCh := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if progressDuration, ok := parseFFmpegProgressDuration(scanner.Text()); ok && onProgress != nil {
				onProgress(progressDuration)
			}
		}
		scanErrCh <- scanner.Err()
	}()

	waitErr := cmd.Wait()
	scanErr := <-scanErrCh
	if waitErr == nil && scanErr != nil {
		waitErr = scanErr
	}
	return stderr.Bytes(), waitErr
}

func parseFFmpegProgressDuration(line string) (time.Duration, bool) {
	raw := strings.TrimSpace(line)
	if !strings.HasPrefix(raw, "out_time=") {
		return 0, false
	}
	value := strings.TrimSpace(strings.TrimPrefix(raw, "out_time="))
	if value == "" || value == "N/A" {
		return 0, false
	}

	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return 0, false
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, false
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, false
	}

	secondPart := parts[2]
	wholeSeconds := secondPart
	fractional := ""
	if strings.Contains(secondPart, ".") {
		wholeSeconds, fractional, _ = strings.Cut(secondPart, ".")
	}
	seconds, err := strconv.Atoi(wholeSeconds)
	if err != nil {
		return 0, false
	}

	nanoseconds := 0
	if fractional != "" {
		fractional = strings.TrimSpace(fractional)
		if len(fractional) > 9 {
			fractional = fractional[:9]
		}
		fractional = fractional + strings.Repeat("0", 9-len(fractional))
		nanoseconds, err = strconv.Atoi(fractional)
		if err != nil {
			return 0, false
		}
	}

	duration := time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		time.Duration(nanoseconds)
	return duration, true
}
