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

func TestStatusIncludesActiveDownloads(t *testing.T) {
	manager := NewManager()
	info := twitcasting.MovieDownloadInfo{
		ScreenID:     "mielu_ii",
		MovieID:      "833944593",
		StreamerName: "ミエル",
		Title:        "會員回放 ／ Test",
		PlaylistURLs: []string{"https://dl.example.test/master.m3u8"},
	}

	if err := manager.handleDownloadStart("mielu_ii|833944593", info, "Recordings/mielu_ii"); err != nil {
		t.Fatalf("handleDownloadStart() error = %v", err)
	}
	manager.handleDownloadProgress("mielu_ii|833944593", twitcasting.MovieDownloadProgress{
		PartIndex:       0,
		PartCount:       1,
		OutputPath:      "Recordings/mielu_ii/[ミエル][2026-04-17]會員回放 ／ Test.mp4",
		ProgressPercent: 42.5,
	})

	status := manager.Status()
	if len(status.ActiveDownloads) != 1 {
		t.Fatalf("len(status.ActiveDownloads) = %d, want 1", len(status.ActiveDownloads))
	}
	active := status.ActiveDownloads[0]
	if active.Streamer != "mielu_ii" {
		t.Fatalf("active.Streamer = %q", active.Streamer)
	}
	if active.MovieID != "833944593" {
		t.Fatalf("active.MovieID = %q", active.MovieID)
	}
	if active.CurrentPart != 1 || active.TotalParts != 1 {
		t.Fatalf("active part = %d/%d", active.CurrentPart, active.TotalParts)
	}
	if active.ProgressPercent != 42.5 {
		t.Fatalf("active.ProgressPercent = %v, want 42.5", active.ProgressPercent)
	}
	if active.CurrentFile != "[ミエル][2026-04-17]會員回放 ／ Test.mp4" {
		t.Fatalf("active.CurrentFile = %q", active.CurrentFile)
	}
}
