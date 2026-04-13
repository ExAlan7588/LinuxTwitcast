package discord

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendTestMessage(t *testing.T) {
	originalAPI := discordAPI
	defer func() {
		discordAPI = originalAPI
	}()

	var gotAuth string
	var gotPath string
	var gotMethod string
	var gotPayload map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"123"}`))
	}))
	defer server.Close()

	discordAPI = server.URL

	err := SendTestMessage(Config{
		BotToken:        "bot-token",
		NotifyChannelID: "channel-123",
	}, "hello from test")
	if err != nil {
		t.Fatalf("SendTestMessage returned error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %s, want %s", gotMethod, http.MethodPost)
	}
	if gotPath != "/channels/channel-123/messages" {
		t.Fatalf("path = %s, want %s", gotPath, "/channels/channel-123/messages")
	}
	if gotAuth != "Bot bot-token" {
		t.Fatalf("authorization = %q, want %q", gotAuth, "Bot bot-token")
	}
	if gotPayload["content"] != "hello from test" {
		t.Fatalf("content = %q, want %q", gotPayload["content"], "hello from test")
	}
}
