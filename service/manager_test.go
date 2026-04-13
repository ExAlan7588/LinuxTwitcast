package service

import (
	"testing"
	"time"

	"github.com/jzhang046/croned-twitcasting-recorder/twitcasting"
)

func TestHandleStreamLookupTracksPasswordWarnings(t *testing.T) {
	manager := NewManager()

	manager.handleStreamLookup("locked-user", twitcasting.ErrPasswordRequired)

	status := manager.Status()
	if len(status.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(status.Warnings))
	}
	if status.Warnings[0].Code != "stream_password_required" {
		t.Fatalf("unexpected warning code: %s", status.Warnings[0].Code)
	}
	if status.Warnings[0].Streamer != "locked-user" {
		t.Fatalf("unexpected warning streamer: %s", status.Warnings[0].Streamer)
	}
	if !manager.shouldSkipStreamer("locked-user") {
		t.Fatal("expected password warning to trigger cooldown skip")
	}
}

func TestHandleStreamLookupClearsWarningWhenOffline(t *testing.T) {
	manager := NewManager()
	manager.warnings["locked-user"] = streamWarning{
		Code:    "stream_password_required",
		RetryAt: time.Now().Add(time.Minute),
	}

	manager.handleStreamLookup("locked-user", twitcasting.ErrStreamOffline)

	if len(manager.Status().Warnings) != 0 {
		t.Fatal("expected offline result to clear password warning")
	}
}

func TestShouldSkipStreamerExpiresAfterRetryTime(t *testing.T) {
	manager := NewManager()
	manager.warnings["locked-user"] = streamWarning{
		Code:    "stream_password_required",
		RetryAt: time.Now().Add(-time.Second),
	}

	if manager.shouldSkipStreamer("locked-user") {
		t.Fatal("expected expired cooldown to stop skipping streamer")
	}
}
