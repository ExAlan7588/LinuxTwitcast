package twitcasting

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type MovieArchiveMetadata struct {
	ScreenID     string
	MovieID      string
	StreamerName string
	Title        string
	AvatarURL    string
	CoverURL     string
	MemberOnly   bool
}

func LookupMovieArchiveMetadata(streamer, movieID string) (MovieArchiveMetadata, error) {
	info, err := fetchMoviePageInfo(streamer, movieID, true, parseMoviePageInfo)
	if err != nil {
		return MovieArchiveMetadata{}, err
	}

	return MovieArchiveMetadata{
		ScreenID:     strings.TrimSpace(streamer),
		MovieID:      strings.TrimSpace(movieID),
		StreamerName: info.streamerName,
		Title:        info.title,
		AvatarURL:    info.avatarURL,
		CoverURL:     info.coverURL,
		MemberOnly:   info.memberOnly,
	}, nil
}

func fetchMoviePageInfo(
	streamer string,
	movieID string,
	includeAuth bool,
	parse func(string, string) streamPageInfo,
) (streamPageInfo, error) {
	screenID := strings.TrimSpace(streamer)
	trimmedMovieID := strings.TrimSpace(movieID)
	if screenID == "" || trimmedMovieID == "" {
		return streamPageInfo{streamerName: streamer}, errors.New("streamer and movieID are required")
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/%s/movie/%s", baseDomain, screenID, trimmedMovieID), nil)
	if err != nil {
		return streamPageInfo{streamerName: streamer}, err
	}
	req.Header.Set("User-Agent", userAgent)
	if includeAuth {
		ApplyAuthToRequest(req)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return streamPageInfo{streamerName: streamer}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return streamPageInfo{streamerName: streamer}, fmt.Errorf("unexpected movie page status: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return streamPageInfo{streamerName: streamer}, err
	}

	return parse(screenID, string(bodyBytes)), nil
}

func parseMoviePageInfo(streamer, bodyStr string) streamPageInfo {
	info := parseStreamPageInfo(streamer, bodyStr)
	if title := extractMovieTitleMeta(bodyStr); title != "" {
		info.title = sanitizeFilename(title)
	}
	if title := extractExtendedTwitterDescriptionTitle(bodyStr, info.title, info.streamerName, streamer); title != "" {
		info.title = sanitizeFilename(title)
	}
	return info
}

func extractMovieTitleMeta(body string) string {
	return extractMetaContentByPatterns(body, movieTitleMetaRegexes)
}
