package telegram

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jzhang046/croned-twitcasting-recorder/record"
)

func TestUploadMethodForPath(t *testing.T) {
	testCases := map[string]UploadMethod{
		"recording.m4a": UploadMethodAudio,
		"recording.mp3": UploadMethodAudio,
		"recording.ts":  UploadMethodDocument,
		"notes.txt":     UploadMethodDocument,
	}

	for path, want := range testCases {
		if got := uploadMethodForPath(path); got != want {
			t.Fatalf("uploadMethodForPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestSendTestMessage(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotChatID string
	var gotText string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		gotChatID = r.Form.Get("chat_id")
		gotText = r.Form.Get("text")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	err := SendTestMessage(Config{
		BotToken:    "bot-token",
		ChatID:      "chat-123",
		ApiEndpoint: server.URL,
	}, "hello telegram")
	if err != nil {
		t.Fatalf("SendTestMessage returned error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %s, want %s", gotMethod, http.MethodPost)
	}
	if gotPath != "/botbot-token/sendMessage" {
		t.Fatalf("path = %s, want %s", gotPath, "/botbot-token/sendMessage")
	}
	if gotChatID != "chat-123" {
		t.Fatalf("chat_id = %q, want %q", gotChatID, "chat-123")
	}
	if gotText != "hello telegram" {
		t.Fatalf("text = %q, want %q", gotText, "hello telegram")
	}
}

func TestTaggedM4APathUsesArtistDateAndTitle(t *testing.T) {
	session := record.SessionInfo{
		Filename:     `C:\recordings\source.ts`,
		StreamerName: "測試主播",
		Title:        "今晚雜談",
		StartedAt:    time.Date(2026, 4, 14, 23, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60)),
	}

	got := taggedM4APath(session)
	want := `C:\recordings\[測試主播][2026-04-14]今晚雜談.m4a`
	if got != want {
		t.Fatalf("taggedM4APath() = %q, want %q", got, want)
	}
}

func TestBuildM4AArgsIncludesMetadataAndCover(t *testing.T) {
	session := record.SessionInfo{
		Filename:     `C:\recordings\source.ts`,
		StreamerName: "測試主播",
		Title:        "今晚雜談",
		StartedAt:    time.Date(2026, 4, 14, 9, 0, 0, 0, time.UTC),
	}

	args := buildM4AArgs(session, session.Filename, `C:\recordings\out.m4a`, `C:\temp\cover.jpg`)
	joined := strings.Join(args, " ")

	for _, needle := range []string{
		"-i C:\\recordings\\source.ts",
		"-i C:\\temp\\cover.jpg",
		"-disposition:v:0 attached_pic",
		"-metadata artist=測試主播",
		"-metadata title=今晚雜談",
		"-metadata date=2026-04-14",
		"-metadata:s:v title=Cover",
		`C:\recordings\out.m4a`,
	} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("expected args to contain %q, got %q", needle, joined)
		}
	}
}

func TestProcessSkipsUploadWhenFFmpegFails(t *testing.T) {
	originalRunFFmpeg := runFFmpeg
	originalUploadTelegramFile := uploadTelegramFile
	defer func() {
		runFFmpeg = originalRunFFmpeg
		uploadTelegramFile = originalUploadTelegramFile
	}()

	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, "source.ts")
	if err := os.WriteFile(sourceFile, []byte("dummy"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	uploadCalls := 0
	runFFmpeg = func(args ...string) ([]byte, error) {
		return []byte("ffmpeg failed"), errors.New("boom")
	}
	uploadTelegramFile = func(cfg Config, filePath string, caption string) error {
		uploadCalls++
		return nil
	}

	Process(Config{
		Enabled:      true,
		BotToken:     "bot-token",
		ChatID:       "chat-id",
		ConvertToM4A: true,
	}, record.SessionInfo{
		Filename:     sourceFile,
		StreamerName: "測試主播",
		Title:        "今晚雜談",
		StartedAt:    time.Date(2026, 4, 14, 9, 0, 0, 0, time.UTC),
	})

	if uploadCalls != 0 {
		t.Fatalf("expected no Telegram upload when ffmpeg fails, got %d calls", uploadCalls)
	}
	if _, err := os.Stat(sourceFile); err != nil {
		t.Fatalf("expected original file to remain after ffmpeg failure, stat err: %v", err)
	}
}
