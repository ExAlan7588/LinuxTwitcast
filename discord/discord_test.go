package discord

import (
	"strings"
	"testing"
	"time"

	"github.com/jzhang046/croned-twitcasting-recorder/record"
)

func TestBuildStartEmbedUsesPlainRedDotStatus(t *testing.T) {
	embed := buildStartEmbedWithTexts(record.SessionInfo{
		Streamer:     "screen-id",
		StreamerName: "主播",
		Title:        "title",
		AvatarURL:    "https://example.com/avatar.jpg",
		CoverURL:     "https://example.com/cover.jpg",
	}, 5*time.Second, textsForLanguage(discordLangChinese))
	if len(embed.Fields) == 0 {
		t.Fatal("expected status fields in start embed")
	}

	statusValue := embed.Fields[0].Value
	if strings.Contains(statusValue, "<a:live:") {
		t.Fatalf("expected no literal custom emoji tag in status field, got %q", statusValue)
	}
	if !strings.Contains(statusValue, "🔴") {
		t.Fatalf("expected red dot in status field, got %q", statusValue)
	}
	if embed.Thumbnail == nil || embed.Thumbnail.Url != "https://example.com/avatar.jpg" {
		t.Fatalf("expected thumbnail avatar to be set, got %+v", embed.Thumbnail)
	}
	if embed.Image == nil || embed.Image.Url != "https://example.com/cover.jpg" {
		t.Fatalf("expected cover image to be set, got %+v", embed.Image)
	}
	for _, field := range embed.Fields {
		if field.Name == "直播頁面" {
			t.Fatalf("expected live page field to be removed, got %+v", embed.Fields)
		}
	}
}

func TestBuildEndEmbedAddsTelegramArchiveLink(t *testing.T) {
	embed := buildEndEmbedWithTexts(record.SessionInfo{
		Streamer:     "screen-id",
		StreamerName: "主播",
		Title:        "title",
		AvatarURL:    "https://example.com/avatar.jpg",
		CoverURL:     "https://example.com/cover.jpg",
	}, 38*time.Second, "https://t.me/archive/123", textsForLanguage(discordLangChinese))

	if embed.Image == nil || embed.Image.Url != "https://example.com/cover.jpg" {
		t.Fatalf("expected cover image to be set, got %+v", embed.Image)
	}

	lastField := embed.Fields[len(embed.Fields)-1]
	if lastField.Name != "錄播檔案" {
		t.Fatalf("expected archive link field at the bottom, got %+v", embed.Fields)
	}
	if !strings.Contains(lastField.Value, "https://t.me/archive/123") {
		t.Fatalf("expected telegram link in archive field, got %q", lastField.Value)
	}
}

func TestBuildStartEmbedMarksMembersOnlySessions(t *testing.T) {
	embed := buildStartEmbedWithTexts(record.SessionInfo{
		Streamer:     "screen-id",
		MovieID:      "834555312",
		StreamerName: "主播",
		MemberOnly:   true,
	}, 5*time.Second, textsForLanguage(discordLangChinese))

	if !strings.HasPrefix(embed.Title, "【會員限定】") {
		t.Fatalf("expected members-only title prefix, got %q", embed.Title)
	}
	if !strings.Contains(embed.Title, "會員限定直播") {
		t.Fatalf("expected members-only fallback title, got %q", embed.Title)
	}

	var liveType, movieID string
	for _, item := range embed.Fields {
		switch item.Name {
		case "直播類型":
			liveType = item.Value
		case "直播編號":
			movieID = item.Value
		}
	}

	if !strings.Contains(liveType, "會員限定") {
		t.Fatalf("expected members-only field, got %+v", embed.Fields)
	}
	if !strings.Contains(movieID, "834555312") {
		t.Fatalf("expected movie id field, got %+v", embed.Fields)
	}
}

func TestFormatTitleSanitizesReservedCharacters(t *testing.T) {
	formatted := FormatTitle(`主/播`, `標題? #1`)
	if strings.Contains(formatted, "/") || strings.Contains(formatted, "?") || strings.Contains(formatted, "#") {
		t.Fatalf("expected reserved ASCII punctuation to be sanitized, got %q", formatted)
	}
	if !strings.Contains(formatted, "主／播") || !strings.Contains(formatted, "標題？ ＃1") {
		t.Fatalf("unexpected sanitized title output: %q", formatted)
	}
}

func TestBuildStartEmbedUsesEnglishWhenConfigured(t *testing.T) {
	embed := buildStartEmbedWithTexts(record.SessionInfo{
		Streamer:     "screen-id",
		StreamerName: "Streamer",
		Title:        "Title",
		MemberOnly:   true,
	}, 5*time.Second, textsForLanguage(discordLangEnglish))

	if !strings.HasPrefix(embed.Title, "[Members Only] ") {
		t.Fatalf("expected english members-only prefix, got %q", embed.Title)
	}
	if embed.Fields[0].Name != "Status" {
		t.Fatalf("expected english field name, got %+v", embed.Fields)
	}
	if !strings.Contains(embed.Fields[0].Value, "Recording in progress") {
		t.Fatalf("expected english status text, got %+v", embed.Fields)
	}
}
