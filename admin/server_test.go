package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jzhang046/croned-twitcasting-recorder/applog"
	"github.com/jzhang046/croned-twitcasting-recorder/config"
	"github.com/jzhang046/croned-twitcasting-recorder/record"
	"github.com/jzhang046/croned-twitcasting-recorder/service"
	"github.com/jzhang046/croned-twitcasting-recorder/telegram"
	"github.com/jzhang046/croned-twitcasting-recorder/twitcasting"
)

func TestIsPublicListen(t *testing.T) {
	testCases := []struct {
		name string
		addr string
		want bool
	}{
		{
			name: "empty host treated as public",
			addr: "",
			want: true,
		},
		{
			name: "ipv4 all interface",
			addr: "0.0.0.0:8080",
			want: true,
		},
		{
			name: "ipv6 wildcard",
			addr: "[::]:8080",
			want: true,
		},
		{
			name: "localhost blocked",
			addr: "localhost:8080",
			want: false,
		},
		{
			name: "ipv4 loopback blocked",
			addr: "127.0.0.1:8080",
			want: false,
		},
		{
			name: "ipv6 loopback blocked",
			addr: "[::1]:8080",
			want: false,
		},
		{
			name: "private ip treated public for safety",
			addr: "192.168.1.10:8080",
			want: true,
		},
		{
			name: "malformed address treated as public",
			addr: "%%%bad%%%",
			want: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsPublicListen(tc.addr); got != tc.want {
				t.Fatalf("IsPublicListen(%q) = %v, want %v", tc.addr, got, tc.want)
			}
		})
	}
}

func TestHandleBotRestartSchedulesRestart(t *testing.T) {
	restartRequested := make(chan struct{}, 1)
	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: t.TempDir(),
	}, service.NewManager(), restartRequested)

	req := httptest.NewRequest(http.MethodPost, "/api/bot/restart", nil)
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	select {
	case <-restartRequested:
	default:
		t.Fatal("expected restart request to be queued")
	}
}

func TestHandleBotRestartRejectsWrongMethod(t *testing.T) {
	restartRequested := make(chan struct{}, 1)
	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: t.TempDir(),
	}, service.NewManager(), restartRequested)

	req := httptest.NewRequest(http.MethodGet, "/api/bot/restart", nil)
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", recorder.Code)
	}
}

func TestHandleFileTelegramUploadUsesDocumentForGenericFiles(t *testing.T) {
	rootDir := t.TempDir()
	recordingsRoot := filepath.Join(rootDir, "Recordings")
	if err := os.MkdirAll(recordingsRoot, 0755); err != nil {
		t.Fatalf("mkdir recordings root: %v", err)
	}
	tempFile := filepath.Join(recordingsRoot, "notes.txt")
	if err := os.WriteFile(tempFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	var receivedPath string
	var receivedChatID string
	telegramServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		receivedPath = r.URL.Path
		receivedChatID = r.FormValue("chat_id")
		fileHeaders := r.MultipartForm.File["document"]
		if len(fileHeaders) != 1 {
			t.Fatalf("expected one document upload, got %d", len(fileHeaders))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer telegramServer.Close()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir temp root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	if err := telegram.SaveConfig(telegram.Config{
		Enabled:     true,
		BotToken:    "test-token",
		ChatID:      "chat-123",
		ApiEndpoint: telegramServer.URL,
	}); err != nil {
		t.Fatalf("save telegram config: %v", err)
	}

	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: rootDir,
	}, service.NewManager(), nil)

	body, err := json.Marshal(map[string]string{
		"root": recordingsRoot,
		"path": "notes.txt",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/files/telegram-upload", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if receivedPath != "/bottest-token/sendDocument" {
		t.Fatalf("unexpected Telegram path: %s", receivedPath)
	}
	if receivedChatID != "chat-123" {
		t.Fatalf("unexpected chat_id: %s", receivedChatID)
	}
}

func TestHandleFileConvertM4ARejectsNonTSFiles(t *testing.T) {
	rootDir := t.TempDir()
	recordingsRoot := filepath.Join(rootDir, "Recordings")
	if err := os.MkdirAll(recordingsRoot, 0755); err != nil {
		t.Fatalf("mkdir recordings root: %v", err)
	}
	tempFile := filepath.Join(recordingsRoot, "notes.txt")
	if err := os.WriteFile(tempFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: rootDir,
	}, service.NewManager(), nil)

	body, err := json.Marshal(map[string]string{
		"root": recordingsRoot,
		"path": "notes.txt",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/files/convert-m4a", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleFileConvertM4AConvertsTSFiles(t *testing.T) {
	rootDir := t.TempDir()
	recordingsRoot := filepath.Join(rootDir, "Recordings")
	if err := os.MkdirAll(recordingsRoot, 0755); err != nil {
		t.Fatalf("mkdir recordings root: %v", err)
	}
	tempFile := filepath.Join(recordingsRoot, "sample.ts")
	if err := os.WriteFile(tempFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	originalConvertManagedMediaFile := convertManagedMediaFile
	convertManagedMediaFile = func(session record.SessionInfo, filePath string) (string, error) {
		if filePath != tempFile {
			t.Fatalf("unexpected conversion path: %s", filePath)
		}
		return filepath.Join(recordingsRoot, "sample.m4a"), nil
	}
	t.Cleanup(func() {
		convertManagedMediaFile = originalConvertManagedMediaFile
	})

	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: rootDir,
	}, service.NewManager(), nil)

	body, err := json.Marshal(map[string]string{
		"root": recordingsRoot,
		"path": "sample.ts",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/files/convert-m4a", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "\"output\":\"sample.m4a\"") {
		t.Fatalf("expected output file in response, got %s", recorder.Body.String())
	}
}

func TestHandleFileConvertM4AConvertsMP4Files(t *testing.T) {
	rootDir := t.TempDir()
	recordingsRoot := filepath.Join(rootDir, "Recordings")
	if err := os.MkdirAll(recordingsRoot, 0755); err != nil {
		t.Fatalf("mkdir recordings root: %v", err)
	}
	tempFile := filepath.Join(recordingsRoot, "sample.mp4")
	if err := os.WriteFile(tempFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	originalConvertManagedMediaFile := convertManagedMediaFile
	convertManagedMediaFile = func(session record.SessionInfo, filePath string) (string, error) {
		if filePath != tempFile {
			t.Fatalf("unexpected conversion path: %s", filePath)
		}
		return filepath.Join(recordingsRoot, "sample.m4a"), nil
	}
	t.Cleanup(func() {
		convertManagedMediaFile = originalConvertManagedMediaFile
	})

	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: rootDir,
	}, service.NewManager(), nil)

	body, err := json.Marshal(map[string]string{
		"root": recordingsRoot,
		"path": "sample.mp4",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/files/convert-m4a", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "\"output\":\"sample.m4a\"") {
		t.Fatalf("expected output file in response, got %s", recorder.Body.String())
	}
}

func TestHandleFileConvertM4AUsesStreamerAvatarWhenFolderMatchesConfig(t *testing.T) {
	rootDir := t.TempDir()
	chdirTestRoot(t, rootDir)

	recordingsRoot := filepath.Join(rootDir, "Recordings")
	targetDir := filepath.Join(recordingsRoot, "mielu")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	tempFile := filepath.Join(targetDir, "sample.ts")
	if err := os.WriteFile(tempFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := config.Save(filepath.Join(rootDir, "config.json"), &config.AppConfig{
		Lang: "ZH",
		Streamers: []*config.StreamerConfig{
			{
				ScreenId: "mielu_ii",
				Schedule: "@every 5s",
				Folder:   "Recordings/mielu",
				Enabled:  true,
			},
		},
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	originalLookup := lookupStreamerProfile
	lookupStreamerProfile = func(screenID string) (twitcasting.StreamerProfile, error) {
		if screenID != "mielu_ii" {
			t.Fatalf("unexpected screenID: %s", screenID)
		}
		return twitcasting.StreamerProfile{
			ScreenID:     "mielu_ii",
			StreamerName: "ミエル",
			AvatarURL:    "https://example.test/avatar.jpg",
		}, nil
	}
	t.Cleanup(func() {
		lookupStreamerProfile = originalLookup
	})

	originalConvertManagedMediaFile := convertManagedMediaFile
	convertManagedMediaFile = func(session record.SessionInfo, filePath string) (string, error) {
		if filePath != tempFile {
			t.Fatalf("unexpected conversion path: %s", filePath)
		}
		if session.Streamer != "mielu_ii" {
			t.Fatalf("session.Streamer = %q", session.Streamer)
		}
		if session.StreamerName != "ミエル" {
			t.Fatalf("session.StreamerName = %q", session.StreamerName)
		}
		if session.AvatarURL != "https://example.test/avatar.jpg" {
			t.Fatalf("session.AvatarURL = %q", session.AvatarURL)
		}
		return filepath.Join(targetDir, "sample.m4a"), nil
	}
	t.Cleanup(func() {
		convertManagedMediaFile = originalConvertManagedMediaFile
	})

	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: rootDir,
	}, service.NewManager(), nil)

	body, err := json.Marshal(map[string]string{
		"root": recordingsRoot,
		"path": "mielu/sample.ts",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/files/convert-m4a", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleVersionCheckReturnsJSON(t *testing.T) {
	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: t.TempDir(),
	}, service.NewManager(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/version/check", nil)
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload VersionCheckResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Version != defaultVersion {
		t.Fatalf("unexpected version: %q", payload.Version)
	}
	if payload.Message == "" {
		t.Fatal("expected a human-readable message")
	}
}

func TestHandleStreamerCheckReturnsProfile(t *testing.T) {
	originalLookup := lookupStreamerProfile
	lookupStreamerProfile = func(screenID string) (twitcasting.StreamerProfile, error) {
		if screenID != "alice" {
			t.Fatalf("unexpected screenID: %s", screenID)
		}
		return twitcasting.StreamerProfile{
			ScreenID:         "alice",
			StreamerName:     "Alice Channel",
			Title:            "Night Stream",
			AvatarURL:        "https://example.test/avatar.jpg",
			PasswordRequired: false,
		}, nil
	}
	t.Cleanup(func() {
		lookupStreamerProfile = originalLookup
	})

	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: t.TempDir(),
	}, service.NewManager(), nil)

	body := bytes.NewBufferString(`{"screen_id":"alice"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/streamers/check", body)
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload["screen_id"] != "alice" {
		t.Fatalf("unexpected screen_id: %#v", payload["screen_id"])
	}
	if payload["streamer_name"] != "Alice Channel" {
		t.Fatalf("unexpected streamer_name: %#v", payload["streamer_name"])
	}
}

func TestHandleStreamerCheckReturnsNotFoundForUnknownScreenID(t *testing.T) {
	originalLookup := lookupStreamerProfile
	lookupStreamerProfile = func(string) (twitcasting.StreamerProfile, error) {
		return twitcasting.StreamerProfile{}, twitcasting.ErrStreamerNotFound
	}
	t.Cleanup(func() {
		lookupStreamerProfile = originalLookup
	})

	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: t.TempDir(),
	}, service.NewManager(), nil)

	body := bytes.NewBufferString(`{"screen_id":"ghost"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/streamers/check", body)
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleStreamerCheckRejectsBlankScreenID(t *testing.T) {
	originalLookup := lookupStreamerProfile
	lookupStreamerProfile = func(string) (twitcasting.StreamerProfile, error) {
		return twitcasting.StreamerProfile{}, errors.New("should not be called")
	}
	t.Cleanup(func() {
		lookupStreamerProfile = originalLookup
	})

	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: t.TempDir(),
	}, service.NewManager(), nil)

	body := bytes.NewBufferString(`{"screen_id":"   "}`)
	req := httptest.NewRequest(http.MethodPost, "/api/streamers/check", body)
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleLogsSupportsErrorsOnlyAndAlertSummary(t *testing.T) {
	rootDir := t.TempDir()
	chdirTestRoot(t, rootDir)

	if err := applog.Configure(false); err != nil {
		t.Fatalf("Configure(false) error = %v", err)
	}
	t.Cleanup(func() {
		_ = applog.Close()
	})

	log.Println("[Info] Streamer [mielu_ii] is currently offline; skipping this polling round")
	log.Println("[Error] ffmpeg exploded")

	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: rootDir,
	}, service.NewManager(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/logs?limit=1&alert_limit=1&hide_offline=1&errors_only=1", nil)
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Lines         []string `json:"lines"`
		FilteredCount int      `json:"filtered_count"`
		AlertLines    []string `json:"alert_lines"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal logs payload: %v", err)
	}
	if len(payload.Lines) != 1 || !strings.Contains(payload.Lines[0], "[Error] ffmpeg exploded") {
		t.Fatalf("unexpected filtered lines: %#v", payload.Lines)
	}
	if payload.FilteredCount < 1 {
		t.Fatalf("expected filtered_count >= 1, got %d", payload.FilteredCount)
	}
	if len(payload.AlertLines) != 1 || !strings.Contains(payload.AlertLines[0], "[Error] ffmpeg exploded") {
		t.Fatalf("unexpected alert_lines: %#v", payload.AlertLines)
	}
}

func TestHandleFileDeleteRemovesNestedDirectoryRecursively(t *testing.T) {
	rootDir := t.TempDir()
	chdirTestRoot(t, rootDir)

	targetDir := filepath.Join(rootDir, "Recordings", "urarachan_u_u", "nested")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "clip.ts"), []byte("test"), 0644); err != nil {
		t.Fatalf("write nested file: %v", err)
	}

	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: rootDir,
	}, service.NewManager(), nil)

	body := bytes.NewBufferString(fmt.Sprintf(`{"root":%q,"path":"urarachan_u_u"}`, filepath.Join(rootDir, "Recordings")))
	req := httptest.NewRequest(http.MethodPost, "/api/files/delete", body)
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if _, err := os.Stat(filepath.Join(rootDir, "Recordings", "urarachan_u_u")); !os.IsNotExist(err) {
		t.Fatalf("expected directory to be removed, stat err=%v", err)
	}
}

func TestHandleFileDeleteRejectsNonRecordingsRoot(t *testing.T) {
	rootDir := t.TempDir()
	chdirTestRoot(t, rootDir)

	targetFile := filepath.Join(rootDir, "notes.txt")
	if err := os.WriteFile(targetFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: rootDir,
	}, service.NewManager(), nil)

	body := bytes.NewBufferString(fmt.Sprintf(`{"root":%q,"path":"notes.txt"}`, rootDir))
	req := httptest.NewRequest(http.MethodPost, "/api/files/delete", body)
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleFilesRejectsProjectRoot(t *testing.T) {
	rootDir := t.TempDir()
	chdirTestRoot(t, rootDir)

	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: rootDir,
	}, service.NewManager(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/files?root="+url.QueryEscape(rootDir), nil)
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleFileDeleteStillRemovesFilesInsideRecordings(t *testing.T) {
	rootDir := t.TempDir()
	chdirTestRoot(t, rootDir)

	recordingsRoot := filepath.Join(rootDir, "Recordings")
	if err := os.MkdirAll(recordingsRoot, 0755); err != nil {
		t.Fatalf("mkdir recordings root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(recordingsRoot, "clip.ts"), []byte("test"), 0644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: rootDir,
	}, service.NewManager(), nil)

	body := bytes.NewBufferString(fmt.Sprintf(`{"root":%q,"path":"clip.ts"}`, recordingsRoot))
	req := httptest.NewRequest(http.MethodPost, "/api/files/delete", body)
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if _, err := os.Stat(filepath.Join(recordingsRoot, "clip.ts")); !os.IsNotExist(err) {
		t.Fatalf("expected file to be removed, stat err=%v", err)
	}
}

func TestHandleTwitCastingAuthStoresCookieStatusWithoutEchoingSecret(t *testing.T) {
	rootDir := t.TempDir()
	chdirTestRoot(t, rootDir)

	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: rootDir,
	}, service.NewManager(), nil)

	body := bytes.NewBufferString(`{"content":"Cookie: tc_ss=session123; tc_id=alice"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/twitcasting/auth", body)
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "session123") {
		t.Fatalf("response leaked cookie secret: %s", recorder.Body.String())
	}
	if _, err := os.Stat(filepath.Join(rootDir, "twitcasting_auth.json")); err != nil {
		t.Fatalf("expected auth file to be created: %v", err)
	}
	if !strings.Contains(recorder.Body.String(), `"configured":true`) {
		t.Fatalf("expected configured auth status, got %s", recorder.Body.String())
	}
}

func TestHandleTwitCastingAuthDeleteClearsCookie(t *testing.T) {
	rootDir := t.TempDir()
	chdirTestRoot(t, rootDir)

	if err := twitcasting.SaveAuthConfig(twitcasting.AuthConfig{CookieHeader: "tc_ss=session123"}); err != nil {
		t.Fatalf("SaveAuthConfig() error = %v", err)
	}

	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: rootDir,
	}, service.NewManager(), nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/twitcasting/auth", nil)
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if _, err := os.Stat(filepath.Join(rootDir, "twitcasting_auth.json")); !os.IsNotExist(err) {
		t.Fatalf("expected auth file to be removed, stat err=%v", err)
	}
	if !strings.Contains(recorder.Body.String(), `"configured":false`) {
		t.Fatalf("expected cleared auth status, got %s", recorder.Body.String())
	}
}

func TestHandleManualRecordQueuesSingleRecording(t *testing.T) {
	originalStartManualRecording := startManualRecording
	startManualRecording = func(manager *service.Manager, rawURL string) (service.ManualRecordResult, error) {
		if rawURL != "https://twitcasting.tv/alice/movie/123" {
			t.Fatalf("unexpected raw URL: %s", rawURL)
		}
		return service.ManualRecordResult{
			Mode:         service.ManualActionModeRecord,
			Streamer:     "alice",
			StreamerName: "Alice Channel",
			Title:        "Night Stream",
			MovieID:      "123",
			Folder:       "Recordings/alice",
		}, nil
	}
	t.Cleanup(func() {
		startManualRecording = originalStartManualRecording
	})

	server := NewServer(Options{
		Address: "127.0.0.1:8080",
		RootDir: t.TempDir(),
	}, service.NewManager(), nil)

	req := httptest.NewRequest(http.MethodPost, "/api/manual/record", bytes.NewBufferString(`{"url":"https://twitcasting.tv/alice/movie/123"}`))
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"queued":true`) {
		t.Fatalf("expected queued response, got %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"mode":"record"`) {
		t.Fatalf("expected manual mode in response, got %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"streamer":"alice"`) {
		t.Fatalf("expected streamer in response, got %s", recorder.Body.String())
	}
}

func chdirTestRoot(t *testing.T, rootDir string) {
	t.Helper()

	twitcasting.InvalidateAuthCache()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir temp root: %v", err)
	}
	t.Cleanup(func() {
		twitcasting.InvalidateAuthCache()
		_ = os.Chdir(originalWD)
	})
}
