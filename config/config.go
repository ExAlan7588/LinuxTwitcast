package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
)

const defaultConfigPath = "config.json"

// StreamerConfig holds per-streamer recording settings.
type StreamerConfig struct {
	ScreenId string `json:"screen-id"`
	Schedule string `json:"schedule"`
	Folder   string `json:"folder"`
	Password string `json:"password,omitempty"`
	Enabled  bool   `json:"enabled"`
}

// AppConfig stores both recorder settings and lightweight UI preferences.
type AppConfig struct {
	Lang      string            `json:"lang,omitempty"`
	Streamers []*StreamerConfig `json:"streamers"`
	EnableLog bool              `json:"enable_log"`
}

func Default() *AppConfig {
	return &AppConfig{
		Lang:      "EN",
		Streamers: []*StreamerConfig{},
	}
}

func GetDefaultConfig() *AppConfig {
	cfg, err := LoadDefault()
	if err != nil {
		log.Fatal("Error parsing config file:\n", err)
	}
	return cfg
}

func LoadDefault() (*AppConfig, error) {
	return Load(defaultConfigPath)
}

func SaveDefault(cfg *AppConfig) error {
	return Save(defaultConfigPath, cfg)
}

func Load(configPath string) (*AppConfig, error) {
	configData, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, err
	}

	cfg := Default()
	if err := json.Unmarshal(configData, cfg); err != nil {
		return nil, err
	}

	cfg = normalize(cfg)
	return cfg, Validate(cfg)
}

func Save(configPath string, cfg *AppConfig) error {
	if cfg == nil {
		return errors.New("config is required")
	}

	normalized := normalize(cfg)
	if err := Validate(normalized); err != nil {
		return err
	}

	configData, err := json.MarshalIndent(normalized, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, configData, 0644)
}

func Validate(cfg *AppConfig) error {
	if cfg == nil {
		return errors.New("config is required")
	}

	for idx, streamer := range cfg.Streamers {
		if streamer == nil {
			return fmt.Errorf("streamers[%d] is required", idx)
		}
		if strings.TrimSpace(streamer.ScreenId) == "" {
			return fmt.Errorf("streamers[%d].screen-id is required", idx)
		}
		if strings.TrimSpace(streamer.Schedule) == "" {
			return fmt.Errorf("streamers[%d].schedule is required", idx)
		}
	}

	return nil
}

func EnabledStreamers(cfg *AppConfig) int {
	if cfg == nil {
		return 0
	}

	enabled := 0
	for _, streamer := range cfg.Streamers {
		if streamer != nil && streamer.Enabled {
			enabled++
		}
	}
	return enabled
}

func normalize(cfg *AppConfig) *AppConfig {
	if cfg == nil {
		return Default()
	}

	normalized := &AppConfig{
		Lang:      strings.TrimSpace(cfg.Lang),
		Streamers: make([]*StreamerConfig, 0, len(cfg.Streamers)),
		EnableLog: cfg.EnableLog,
	}
	if normalized.Lang == "" {
		normalized.Lang = "EN"
	}

	for _, streamer := range cfg.Streamers {
		if streamer == nil {
			continue
		}
		normalized.Streamers = append(normalized.Streamers, &StreamerConfig{
			ScreenId: strings.TrimSpace(streamer.ScreenId),
			Schedule: strings.TrimSpace(streamer.Schedule),
			Folder:   strings.TrimSpace(streamer.Folder),
			Password: strings.TrimSpace(streamer.Password),
			Enabled:  streamer.Enabled,
		})
	}

	return normalized
}
