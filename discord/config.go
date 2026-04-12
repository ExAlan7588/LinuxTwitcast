package discord

import (
	"encoding/json"
	"log"
	"os"
)

const discordConfigPath = "discord.json"

// Config holds all Discord bot credentials and notification settings.
// This file is kept separate from config.json and should NOT be committed to version control.
type Config struct {
	Enabled          bool   `json:"enabled"`
	BotToken         string `json:"bot_token"`
	GuildID          string `json:"guild_id"`
	NotifyChannelID  string `json:"notify_channel_id"`
	ArchiveChannelID string `json:"archive_channel_id"`
	TagRole          bool   `json:"tag_role"`
}

// LoadConfig reads discord.json and returns the Discord configuration.
// Returns a disabled (zero-value) config if the file does not exist.
func LoadConfig() Config {
	data, err := os.ReadFile(discordConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("[Discord] discord.json not found — notifications disabled")
			return Config{}
		}
		log.Printf("[Discord] Failed to read discord.json: %v\n", err)
		return Config{}
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("[Discord] Failed to parse discord.json: %v\n", err)
		return Config{}
	}
	return cfg
}

func SaveConfig(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(discordConfigPath, data, 0600)
}
