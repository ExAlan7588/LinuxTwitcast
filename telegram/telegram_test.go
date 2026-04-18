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
		CoverURL:     "https://example.test/cover.jpg",
	}

	args := buildM4AArgs(session, session.Filename, `C:\recordings\out.m4a`, `C:\temp\cover.jpg`, false, false)
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

func TestSessionCoverArtURLPrefersAvatarOverStreamCover(t *testing.T) {
	session := record.SessionInfo{
		AvatarURL: "https://example.test/avatar.jpg",
		CoverURL:  "https://example.test/stream-cover.jpg",
	}

	if got := sessionCoverArtURL(session); got != "https://example.test/avatar.jpg" {
		t.Fatalf("sessionCoverArtURL() = %q, want avatar URL", got)
	}
}

func TestSessionCoverArtURLFallsBackToStreamCover(t *testing.T) {
	session := record.SessionInfo{
		CoverURL: "https://example.test/stream-cover.jpg",
	}

	if got := sessionCoverArtURL(session); got != "https://example.test/stream-cover.jpg" {
		t.Fatalf("sessionCoverArtURL() = %q, want cover URL", got)
	}
}

func TestProcessRetriesConversionStrategiesAfterCopyFailure(t *testing.T) {
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
	ffmpegCalls := 0
	runFFmpeg = func(args ...string) ([]byte, error) {
		ffmpegCalls++
		joined := strings.Join(args, " ")
		if ffmpegCalls == 1 {
			if !strings.Contains(joined, "-c:a copy") {
				t.Fatalf("expected first attempt to use copy audio, got %q", joined)
			}
			return []byte("copy failed"), errors.New("boom")
		}
		if !strings.Contains(joined, "-f mpegts") {
			t.Fatalf("expected second attempt to force mpegts, got %q", joined)
		}
		return []byte("ok"), nil
	}
	uploadTelegramFile = func(cfg Config, filePath string, caption string) (UploadResult, error) {
		uploadCalls++
		return UploadResult{}, nil
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

	if uploadCalls != 1 {
		t.Fatalf("expected upload to continue after fallback conversion, got %d calls", uploadCalls)
	}
	if ffmpegCalls != 2 {
		t.Fatalf("expected 2 ffmpeg attempts, got %d", ffmpegCalls)
	}
	if _, err := os.Stat(sourceFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected original file to be removed after successful conversion, stat err: %v", err)
	}
}

func TestConvertManagedMediaFileRejectsUnsupportedExtension(t *testing.T) {
	if _, err := ConvertManagedMediaFile(record.SessionInfo{}, "recording.mp3"); err == nil {
		t.Fatal("expected non-ts conversion to fail")
	}
}

func TestConvertManagedMediaFileAcceptsMP4(t *testing.T) {
	originalRunFFmpeg := runFFmpeg
	runFFmpeg = func(args ...string) ([]byte, error) {
		return []byte("ok"), nil
	}
	t.Cleanup(func() {
		runFFmpeg = originalRunFFmpeg
	})

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "recording.mp4")
	if err := os.WriteFile(inputFile, []byte("dummy"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	outputFile, err := ConvertManagedMediaFile(record.SessionInfo{}, inputFile)
	if err != nil {
		t.Fatalf("ConvertManagedMediaFile() error = %v", err)
	}
	if outputFile != filepath.Join(tempDir, "recording.m4a") {
		t.Fatalf("outputFile = %q", outputFile)
	}
}

func TestConvertManagedMediaFileUsesProvidedAvatarSession(t *testing.T) {
	originalRunFFmpeg := runFFmpeg
	runFFmpeg = func(args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "-map 1:v:0") || !strings.Contains(joined, "-disposition:v:0 attached_pic") {
			t.Fatalf("expected attached avatar artwork args, got %q", joined)
		}
		return []byte("ok"), nil
	}
	t.Cleanup(func() {
		runFFmpeg = originalRunFFmpeg
	})

	avatarServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/avatar.jpg" {
			t.Fatalf("unexpected cover request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("jpeg"))
	}))
	defer avatarServer.Close()

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "recording.mp4")
	if err := os.WriteFile(inputFile, []byte("dummy"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ConvertManagedMediaFile(record.SessionInfo{
		StreamerName: "ミエル",
		AvatarURL:    avatarServer.URL + "/avatar.jpg",
		CoverURL:     "https://example.test/stream-cover.jpg",
	}, inputFile); err != nil {
		t.Fatalf("ConvertManagedMediaFile() error = %v", err)
	}
}

func TestUploadFileWithResultReturnsMessageURL(t *testing.T) {
	tempDir := t.TempDir()
	audioFile := filepath.Join(tempDir, "recording.m4a")
	if err := os.WriteFile(audioFile, []byte("dummy"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":321,"chat":{"id":-1001234567890,"username":"archive_channel"}}}`))
	}))
	defer server.Close()

	result, err := UploadFileWithResult(Config{
		BotToken:    "bot-token",
		ChatID:      "-1001234567890",
		ApiEndpoint: server.URL,
	}, audioFile, "caption")
	if err != nil {
		t.Fatalf("UploadFileWithResult returned error: %v", err)
	}

	if result.Method != UploadMethodAudio {
		t.Fatalf("method = %q, want %q", result.Method, UploadMethodAudio)
	}
	if result.MessageURL != "https://t.me/archive_channel/321" {
		t.Fatalf("messageURL = %q, want %q", result.MessageURL, "https://t.me/archive_channel/321")
	}
}

func TestTelegramMessageURLSupportsPrivateChat(t *testing.T) {
	got := telegramMessageURL(telegramChat{ID: -100987654321}, 456)
	if got != "https://t.me/c/987654321/456" {
		t.Fatalf("telegramMessageURL() = %q, want %q", got, "https://t.me/c/987654321/456")
	}
}
