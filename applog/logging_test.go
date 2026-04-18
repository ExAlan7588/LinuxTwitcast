package applog

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureWritesAlertLinesToDedicatedErrorLog(t *testing.T) {
	rootDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	if err := Configure(false); err != nil {
		t.Fatalf("Configure(false) error = %v", err)
	}
	t.Cleanup(func() {
		_ = Close()
	})

	log.Println("[Info] Streamer [alice] is currently offline; skipping this polling round")
	log.Println("[Error] downloader crashed")

	alerts := RecentAlertLines(1)
	if len(alerts) != 1 || !strings.Contains(alerts[0], "[Error] downloader crashed") {
		t.Fatalf("RecentAlertLines() = %#v", alerts)
	}

	errorLog, err := os.ReadFile(filepath.Join(rootDir, "error.log"))
	if err != nil {
		t.Fatalf("ReadFile(error.log) error = %v", err)
	}
	if !strings.Contains(string(errorLog), "[Error] downloader crashed") {
		t.Fatalf("error.log missing alert line: %s", string(errorLog))
	}
	if strings.Contains(string(errorLog), "currently offline") {
		t.Fatalf("error.log should not contain offline info: %s", string(errorLog))
	}
}

func TestIsAlertLineClassifiesErrorAndFatalPatterns(t *testing.T) {
	if !IsAlertLine("[Error] ffmpeg failed hard") {
		t.Fatal("expected [Error] line to be classified as alert")
	}
	if !IsAlertLine("fatal: unexpected shutdown") {
		t.Fatal("expected fatal line to be classified as alert")
	}
	if IsAlertLine("[Info] Streamer [alice] is currently offline; skipping this polling round") {
		t.Fatal("expected offline info line to stay non-alert")
	}
}
