package twitcasting

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/jmoiron/jsonq"
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

func GetWSStreamUrl(streamer string) (string, string, string, error) {
	streamerName, title := fetchStreamInfo(streamer)

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
		return "", streamerName, title, fmt.Errorf("requesting stream info failed: %w", err)
	}
	defer response.Body.Close()

	responseData := map[string]interface{}{}
	if err = json.NewDecoder(response.Body).Decode(&responseData); err != nil {
		return "", streamerName, title, err
	}
	jq := jsonq.NewQuery(responseData)

	if err = checkStreamOnline(jq); err != nil {
		return "", streamerName, title, err
	}

	// Try to get URL directly
	if streamUrl, err := getDirectStreamUrl(jq); err == nil {
		return streamUrl, streamerName, title, nil
	}

	log.Printf("Direct Stream URL for streamer [%s] not available in the API response; fallback to default URL\n", streamer)
	fallbackUrl, fallbackErr := fallbackStreamUrl(jq)
	return fallbackUrl, streamerName, title, fallbackErr
}

func checkStreamOnline(jq *jsonq.JsonQuery) error {
	isLive, err := jq.Bool("movie", "live")
	if err != nil {
		return fmt.Errorf("error checking stream online status: %w", err)
	} else if !isLive {
		return fmt.Errorf("live stream is offline")
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

func fetchStreamInfo(streamer string) (streamerName string, title string) {
	streamerName = streamer // fallback
	title = ""              // fallback

	req, err := http.NewRequest(http.MethodGet, fmt.Sprint(baseDomain, "/", streamer), nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	bodyStr := string(bodyBytes)

	// --- Extract streamer display name ---
	// TwitCasting <title> when live: "STREAM_TITLE - NAME (@screen-id) - Twitcast"
	// TwitCasting <title> when offline: "NAME (@screen-id) 's Live - Twitcast"
	// The most reliable pattern is "NAME (@screen-id)" inside the <title>.
	nameFromTitleRegex := regexp.MustCompile(`(?i)([^<>]+?)\s*\(@` + regexp.QuoteMeta(streamer) + `\)`)
	titleTagRegex := regexp.MustCompile(`<title>(.*?)</title>`)
	titleMatches := titleTagRegex.FindStringSubmatch(bodyStr)
	if len(titleMatches) > 1 {
		rawTitle := titleMatches[1]
		nameMatch := nameFromTitleRegex.FindStringSubmatch(rawTitle)
		if len(nameMatch) > 1 {
			streamerName = strings.TrimSpace(nameMatch[1])
		}
	}

	// Fallback: try <meta name="author"> for streamer name
	if streamerName == streamer {
		authorRegex := regexp.MustCompile(`<meta\s+name="author"\s+content="([^"]+)"`)
		authorMatch := authorRegex.FindStringSubmatch(bodyStr)
		if len(authorMatch) > 1 {
			candidate := strings.TrimSpace(authorMatch[1])
			if candidate != "" && !strings.EqualFold(candidate, "twitcasting") {
				streamerName = candidate
			}
		}
	}

	// --- Extract stream title ---
	// On TwitCasting, <meta name="twitter:title"> contains the live stream title
	// set by the streamer (e.g. "♡"), NOT the streamer name.
	twitterTitleRegex := regexp.MustCompile(`<meta\s+name="twitter:title"\s+content="([^"]*)"`)
	twitterTitleMatch := twitterTitleRegex.FindStringSubmatch(bodyStr)
	if len(twitterTitleMatch) > 1 {
		candidate := strings.TrimSpace(twitterTitleMatch[1])
		// Only use it as the stream title if it doesn't look like the full page title
		if candidate != "" && !strings.Contains(candidate, "@"+streamer) {
			title = candidate
		}
	}

	// Fallback: try to extract stream title from <title> tag
	// Pattern: "STREAM_TITLE - NAME (@screen-id)"
	if title == "" && len(titleMatches) > 1 {
		rawTitle := titleMatches[1]
		// If the raw title contains " - NAME", the part before it is the stream title
		if streamerName != streamer && strings.Contains(rawTitle, " - "+streamerName) {
			parts := strings.SplitN(rawTitle, " - "+streamerName, 2)
			candidate := strings.TrimSpace(parts[0])
			// Don't treat the streamer name line itself as a title
			if candidate != "" && candidate != streamerName {
				title = candidate
			}
		}
	}

	return sanitizeFilename(streamerName), sanitizeFilename(title)
}
