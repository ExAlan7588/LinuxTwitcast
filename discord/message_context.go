package discord

import (
	"fmt"
	"net/url"
	"strings"
)

type resolvedMessage struct {
	Embeds []embed `json:"embeds"`
}

func resolveScreenIDFromTargetMessage(data interactionCommandData) (string, error) {
	targetID := strings.TrimSpace(data.TargetID)
	if targetID == "" {
		return "", fmt.Errorf("target message ID is empty")
	}

	message, ok := data.Resolved.Messages[targetID]
	if !ok {
		return "", fmt.Errorf("target message %q missing from resolved payload", targetID)
	}

	for _, item := range message.Embeds {
		screenID, err := extractScreenIDFromEmbed(item)
		if err == nil {
			return screenID, nil
		}
	}

	return "", fmt.Errorf("no TwitCasting stream URL found in target message embeds")
}

func extractScreenIDFromEmbed(item embed) (string, error) {
	screenID, err := extractScreenIDFromStreamURL(item.Url)
	if err == nil {
		return screenID, nil
	}
	if item.Author == nil {
		return "", err
	}
	return extractScreenIDFromStreamURL(item.Author.Url)
}

func extractScreenIDFromStreamURL(rawURL string) (string, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("parse stream URL: %w", err)
	}

	host := strings.ToLower(parsedURL.Hostname())
	if !isTwitCastingHost(host) {
		return "", fmt.Errorf("unexpected stream host %q", host)
	}

	path := strings.Trim(parsedURL.EscapedPath(), "/")
	if path == "" {
		return "", fmt.Errorf("stream URL path is empty")
	}

	screenID, _, _ := strings.Cut(path, "/")
	if screenID == "" {
		return "", fmt.Errorf("screen ID is empty")
	}
	return screenID, nil
}

func isTwitCastingHost(host string) bool {
	return host == "twitcasting.tv" || strings.HasSuffix(host, ".twitcasting.tv")
}
