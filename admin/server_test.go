package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jzhang046/croned-twitcasting-recorder/service"
	"github.com/jzhang046/croned-twitcasting-recorder/telegram"
)

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
	tempFile := filepath.Join(rootDir, "notes.txt")
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
		"root": rootDir,
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
