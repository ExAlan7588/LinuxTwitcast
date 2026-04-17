package discord

import (
	"strings"
	"testing"
	"time"

	"github.com/jzhang046/croned-twitcasting-recorder/record"
)

func TestBuildStartEmbedUsesPlainRedDotStatus(t *testing.T) {
	embed := buildStartEmbed(record.SessionInfo{
		Streamer:     "screen-id",
		StreamerName: "主播",
		Title:        "title",
		AvatarURL:    "https://example.com/avatar.jpg",
		CoverURL:     "https://example.com/cover.jpg",
	}, 5*time.Second)
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
	embed := buildEndEmbed(record.SessionInfo{
		Streamer:     "screen-id",
		StreamerName: "主播",
		Title:        "title",
		AvatarURL:    "https://example.com/avatar.jpg",
		CoverURL:     "https://example.com/cover.jpg",
	}, 38*time.Second, "https://t.me/archive/123")

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

func TestFormatTitleSanitizesReservedCharacters(t *testing.T) {
	formatted := FormatTitle(`主/播`, `標題? #1`)
	if strings.Contains(formatted, "/") || strings.Contains(formatted, "?") || strings.Contains(formatted, "#") {
		t.Fatalf("expected reserved ASCII punctuation to be sanitized, got %q", formatted)
	}
	if !strings.Contains(formatted, "主／播") || !strings.Contains(formatted, "標題？ ＃1") {
		t.Fatalf("unexpected sanitized title output: %q", formatted)
	}
}
