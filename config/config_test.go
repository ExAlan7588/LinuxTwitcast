package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReturnsNormalizedConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := `{
		"lang": "  ZH  ",
		"enable_log": true,
		"twitcasting_api": {
			"client_id": "  client-id  ",
			"client_secret": "  client-secret  "
		},
		"streamers": [
			{
				"screen-id": "  alice  ",
				"schedule": "  @every 5s  ",
				"folder": "  Recordings/alice  ",
				"password": "  secret  ",
				"enabled": true
			},
			null
		]
	}`
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Lang != "ZH" {
		t.Fatalf("Lang = %q, want %q", cfg.Lang, "ZH")
	}
	if !cfg.EnableLog {
		t.Fatal("expected EnableLog to stay true")
	}
	if cfg.TwitCastingAPI.ClientID != "client-id" || cfg.TwitCastingAPI.ClientSecret != "client-secret" {
		t.Fatalf("unexpected TwitCasting API credentials: %+v", cfg.TwitCastingAPI)
	}
	if len(cfg.Streamers) != 1 {
		t.Fatalf("len(Streamers) = %d, want 1 after dropping nil entries", len(cfg.Streamers))
	}
	streamer := cfg.Streamers[0]
	if streamer.ScreenId != "alice" || streamer.Schedule != "@every 5s" || streamer.Folder != "Recordings/alice" || streamer.Password != "secret" || !streamer.Enabled {
		t.Fatalf("unexpected normalized streamer: %+v", streamer)
	}
}

func TestLoadRejectsPartialTwitCastingCredentials(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := `{
		"twitcasting_api": {
			"client_id": "client-id"
		},
		"streamers": []
	}`
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected Load() to reject incomplete TwitCasting API credentials")
	}
}
