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
}
