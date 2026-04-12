package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jzhang046/croned-twitcasting-recorder/service"
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
