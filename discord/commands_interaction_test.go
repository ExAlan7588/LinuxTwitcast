package discord

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func useDiscordConfigFileForTest(t *testing.T, lang string) {
	t.Helper()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	cfg := Config{Language: lang}
	cfg.SetLanguage(lang)
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
}

func TestHandleInteractionSubscribesUsingResolvedTargetMessage(t *testing.T) {
	originalAPI := discordAPI
	defer func() { discordAPI = originalAPI }()

	var assignedRole bool
	var responseContent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/interactions/interaction-1/token-1/callback":
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode callback payload: %v", err)
			}
			if payload["type"] != float64(5) {
				t.Fatalf("callback type = %v, want deferred type 5", payload["type"])
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/guilds/guild-1/roles":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"role-1","name":"streamer"}]`))
		case r.Method == http.MethodPut && r.URL.Path == "/guilds/guild-1/members/user-1/roles/role-1":
			assignedRole = true
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPatch && r.URL.Path == "/webhooks/app-1/token-1/messages/@original":
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode edit response payload: %v", err)
			}
			responseContent = payload["content"]
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	discordAPI = server.URL

	raw := json.RawMessage(`{
		"id":"interaction-1",
		"token":"token-1",
		"type":2,
		"guild_id":"guild-1",
		"member":{"user":{"id":"user-1","username":"tester"},"roles":[]},
		"data":{
			"name":"Subscribe Stream Alerts",
			"type":3,
			"target_id":"message-1",
			"resolved":{
				"messages":{
					"message-1":{
						"embeds":[
							{"url":"https://twitcasting.tv/streamer"}
						]
					}
				}
			}
		}
	}`)

	HandleInteraction(Config{
		BotToken: "bot-token",
		GuildID:  "guild-1",
	}, "app-1", raw)

	if !assignedRole {
		t.Fatal("expected role assignment request to be sent")
	}
	if !strings.Contains(responseContent, "@streamer") {
		t.Fatalf("response content = %q, want mention of @streamer", responseContent)
	}
}

func TestHandleInteractionRejectsMessageWithoutTwitCastingURL(t *testing.T) {
	originalAPI := discordAPI
	defer func() { discordAPI = originalAPI }()

	var responseContent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/interactions/interaction-2/token-2/callback" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode callback payload: %v", err)
		}
		if payload["type"] != float64(4) {
			t.Fatalf("callback type = %v, want immediate type 4", payload["type"])
		}
		data, ok := payload["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected callback data payload: %#v", payload["data"])
		}
		content, ok := data["content"].(string)
		if !ok {
			t.Fatalf("unexpected callback content payload: %#v", data["content"])
		}
		responseContent = content
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	discordAPI = server.URL

	raw := json.RawMessage(`{
		"id":"interaction-2",
		"token":"token-2",
		"type":2,
		"guild_id":"guild-1",
		"member":{"user":{"id":"user-1","username":"tester"},"roles":[]},
		"data":{
			"name":"Subscribe Stream Alerts",
			"type":3,
			"target_id":"message-2",
			"resolved":{
				"messages":{
					"message-2":{
						"embeds":[
							{"url":"https://example.com/not-twitcasting"}
						]
					}
				}
			}
		}
	}`)

	HandleInteraction(Config{
		BotToken: "bot-token",
		GuildID:  "guild-1",
	}, "app-1", raw)

	if !strings.Contains(responseContent, "couldn't resolve the streamer") {
		t.Fatalf("response content = %q, want parse failure message", responseContent)
	}
}

func TestHandleInteractionUpdatesLanguageToChinese(t *testing.T) {
	useDiscordConfigFileForTest(t, discordLangEnglish)

	originalAPI := discordAPI
	defer func() { discordAPI = originalAPI }()

	var responseContent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/interactions/interaction-3/token-3/callback":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPut && r.URL.Path == "/applications/app-1/guilds/guild-1/commands":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPatch && r.URL.Path == "/webhooks/app-1/token-3/messages/@original":
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode edit response payload: %v", err)
			}
			responseContent = payload["content"]
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	discordAPI = server.URL

	raw := json.RawMessage(`{
		"id":"interaction-3",
		"token":"token-3",
		"type":2,
		"guild_id":"guild-1",
		"member":{"user":{"id":"user-1","username":"tester"},"roles":[]},
		"data":{
			"name":"lang",
			"type":1,
			"options":[{"name":"mode","type":3,"value":"zh"}]
		}
	}`)

	HandleInteraction(Config{
		BotToken: "bot-token",
		GuildID:  "guild-1",
	}, "app-1", raw)

	if !strings.Contains(responseContent, "繁體中文") {
		t.Fatalf("response content = %q, want Chinese language confirmation", responseContent)
	}

	cfg := LoadConfig()
	if cfg.EffectiveLanguage() != discordLangChinese {
		t.Fatalf("language = %q, want %q", cfg.EffectiveLanguage(), discordLangChinese)
	}
}
