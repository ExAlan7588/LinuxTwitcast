package telegram

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
