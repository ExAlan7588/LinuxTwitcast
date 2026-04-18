package twitcasting

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jzhang046/croned-twitcasting-recorder/record"
)

var ErrMovieDownloadUnsupported = errors.New("the provided movie URL is not currently live; direct movie download is not implemented yet")

type ManualRecordTarget struct {
	ScreenID string
	MovieID  string
	RawURL   string
}

func ParseManualRecordTarget(input string) (ManualRecordTarget, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return ManualRecordTarget{}, errors.New("manual recording URL is required")
	}

	if !strings.Contains(raw, "://") && strings.Contains(strings.ToLower(raw), "twitcasting.tv/") {
		raw = "https://" + strings.TrimPrefix(raw, "//")
	}

	if !strings.ContainsAny(raw, "/?#") {
		return ManualRecordTarget{
			ScreenID: raw,
			RawURL:   fmt.Sprintf("%s/%s", baseDomain, raw),
		}, nil
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return ManualRecordTarget{}, fmt.Errorf("invalid TwitCasting URL: %w", err)
	}
	if parsed.Host != "" && !strings.HasSuffix(strings.ToLower(parsed.Hostname()), "twitcasting.tv") {
		return ManualRecordTarget{}, errors.New("only TwitCasting URLs are supported")
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) == 0 || strings.TrimSpace(segments[0]) == "" {
		return ManualRecordTarget{}, errors.New("failed to parse TwitCasting screen-id from URL")
	}

	target := ManualRecordTarget{
		ScreenID: strings.TrimSpace(segments[0]),
		RawURL:   raw,
	}
	if len(segments) >= 3 && strings.EqualFold(segments[1], "movie") {
		target.MovieID = strings.TrimSpace(segments[2])
	}
	return target, nil
}

func LookupManualRecordingTarget(input, password string) (ManualRecordTarget, record.StreamLookupResult, error) {
	target, err := ParseManualRecordTarget(input)
	if err != nil {
		return ManualRecordTarget{}, record.StreamLookupResult{}, err
	}

	lookup, err := GetWSStreamUrlWithPassword(target.ScreenID, password)
	if target.MovieID == "" {
		return target, lookup, err
	}
	if err == nil && strings.TrimSpace(lookup.MovieID) == target.MovieID {
		return target, lookup, nil
	}
	if err == nil || errors.Is(err, ErrStreamOffline) {
		return target, lookup, ErrMovieDownloadUnsupported
	}
	return target, lookup, err
}
