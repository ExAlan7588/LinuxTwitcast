package service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jzhang046/croned-twitcasting-recorder/record"
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

func TestMemberOnlyTransitionDismissesWhenLookupSucceeds(t *testing.T) {
	manager := NewManager()
	manager.memberOnly["member-user"] = memberOnlyNotification{
		session: record.SessionInfo{Streamer: "member-user"},
	}

	manager.mu.Lock()
	end, dismiss := manager.takeMemberOnlyTransitionLocked("member-user", nil)
	manager.mu.Unlock()

	if end != nil {
		t.Fatal("expected successful lookup to skip member-only archive notification")
	}
	if dismiss == nil {
		t.Fatal("expected successful lookup to dismiss stale member-only message")
	}
	if _, exists := manager.memberOnly["member-user"]; exists {
		t.Fatal("expected member-only state to be cleared")
	}
}

func TestMemberOnlyTransitionArchivesOnlyWhenOffline(t *testing.T) {
	if shouldArchiveMemberOnlyEnd(nil) {
		t.Fatal("expected successful lookup to avoid archive notification")
	}
	if shouldArchiveMemberOnlyEnd(twitcasting.ErrPasswordRequired) {
		t.Fatal("expected password state to avoid member-only archive notification")
	}
	if !shouldArchiveMemberOnlyEnd(twitcasting.ErrStreamOffline) {
		t.Fatal("expected offline lookup to archive member-only ended notification")
	}
}

func TestHandleStreamLookupStoresMemberOnlySession(t *testing.T) {
	manager := NewManager()

	manager.handleStreamLookup("member-user", twitcasting.ErrMemberOnlyLive)

	entry, exists := manager.memberOnly["member-user"]
	if !exists {
		t.Fatal("expected member-only state to be tracked")
	}
	if !entry.session.MemberOnly {
		t.Fatal("expected member-only notification session to be marked")
	}
}

func TestRefreshSessionFromArchiveMetadataRenamesRecording(t *testing.T) {
	originalLookup := lookupMovieArchiveMetadata
	lookupMovieArchiveMetadata = func(streamer, movieID string) (twitcasting.MovieArchiveMetadata, error) {
		if streamer != "mielu_ii" || movieID != "834699167" {
			t.Fatalf("lookupMovieArchiveMetadata(%q, %q)", streamer, movieID)
		}
		return twitcasting.MovieArchiveMetadata{
			StreamerName: "ミエル",
			Title:        "「超かぐや姫」同時視聴♩ ／ ♡ASMR",
			AvatarURL:    "https://image.example/avatar.jpg",
		}, nil
	}
	t.Cleanup(func() {
		lookupMovieArchiveMetadata = originalLookup
	})

	recordingDir := t.TempDir()
	sourcePath := filepath.Join(recordingDir, "[ミエル][2026-05-01]♡ASMR.ts")
	if err := os.WriteFile(sourcePath, []byte("ts data"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	session := record.SessionInfo{
		Streamer:     "mielu_ii",
		MovieID:      "834699167",
		StreamerName: "ミエル",
		Title:        "♡ASMR",
		Filename:     sourcePath,
		StartedAt:    time.Date(2026, 5, 1, 22, 5, 21, 0, time.Local),
	}

	refreshed := refreshSessionFromArchiveMetadata(session)
	wantPath := filepath.Join(recordingDir, "[ミエル][2026-05-01]「超かぐや姫」同時視聴♩ ／ ♡ASMR.ts")
	if refreshed.Filename != wantPath {
		t.Fatalf("Filename = %q, want %q", refreshed.Filename, wantPath)
	}
	if refreshed.Title != "「超かぐや姫」同時視聴♩ ／ ♡ASMR" {
		t.Fatalf("Title = %q", refreshed.Title)
	}
	if refreshed.AvatarURL != "https://image.example/avatar.jpg" {
		t.Fatalf("AvatarURL = %q", refreshed.AvatarURL)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected renamed file: %v", err)
	}
	if _, err := os.Stat(sourcePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected source file to be renamed, stat err = %v", err)
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
