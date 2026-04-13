package discord

import (
	"strings"
	"testing"
	"time"
)

func TestBuildStartEmbedUsesPlainRedDotStatus(t *testing.T) {
	embed := buildStartEmbed("title", "screen-id", 5*time.Second)
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
}
