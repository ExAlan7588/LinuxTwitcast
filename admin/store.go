package admin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/jzhang046/croned-twitcasting-recorder/config"
	"github.com/jzhang046/croned-twitcasting-recorder/discord"
	"github.com/jzhang046/croned-twitcasting-recorder/telegram"
)

type Settings struct {
	App      config.AppConfig `json:"app"`
	Discord  discord.Config   `json:"discord"`
	Telegram telegram.Config  `json:"telegram"`
}

type FileRoot struct {
	Label  string `json:"label"`
	Root   string `json:"root"`
	Exists bool   `json:"exists"`
}

func LoadSettings() (Settings, error) {
	appConfig, err := config.LoadDefault()
	if err != nil {
		return Settings{}, err
	}

	return Settings{
		App:      *appConfig,
		Discord:  discord.LoadConfig(),
		Telegram: telegram.LoadConfig(),
	}, nil
}

func SaveSettings(settings Settings) error {
	if err := ValidateSettings(&settings); err != nil {
		return err
	}
	if err := config.SaveDefault(&settings.App); err != nil {
		return err
	}
	if err := discord.SaveConfig(settings.Discord); err != nil {
		return err
	}
	return telegram.SaveConfig(settings.Telegram)
}

func ValidateSettings(settings *Settings) error {
	if settings == nil {
		return errors.New("settings are required")
	}

	if strings.TrimSpace(settings.App.Lang) == "" {
		settings.App.Lang = "EN"
	}
	if settings.App.Streamers == nil {
		settings.App.Streamers = []*config.StreamerConfig{}
	}
	if err := config.Validate(&settings.App); err != nil {
		return err
	}

	for idx, streamer := range settings.App.Streamers {
		if streamer == nil {
			continue
		}
		if err := validateSchedule(streamer.Schedule); err != nil {
			return fmt.Errorf("streamers[%d].schedule: %w", idx, err)
		}
	}

	if settings.Discord.Enabled {
		if strings.TrimSpace(settings.Discord.BotToken) == "" {
			return errors.New("discord bot token is required when Discord notifications are enabled")
		}
		if strings.TrimSpace(settings.Discord.NotifyChannelID) == "" {
			return errors.New("discord notify channel ID is required when Discord notifications are enabled")
		}
		if settings.Discord.TagRole && strings.TrimSpace(settings.Discord.GuildID) == "" {
			return errors.New("discord guild ID is required when tag_role is enabled")
		}
	}

	if strings.TrimSpace(settings.Telegram.ApiEndpoint) == "" {
		settings.Telegram.ApiEndpoint = "https://api.telegram.org"
	}
	if settings.Telegram.Enabled {
		if strings.TrimSpace(settings.Telegram.BotToken) == "" {
			return errors.New("telegram bot token is required when Telegram uploads are enabled")
		}
		if strings.TrimSpace(settings.Telegram.ChatID) == "" {
			return errors.New("telegram chat_id is required when Telegram uploads are enabled")
		}
	}

	return nil
}

func BuildFileRoots(rootDir string, settings Settings) []FileRoot {
	roots := map[string]FileRoot{}
	addRoot := func(label, candidate string) {
		if strings.TrimSpace(candidate) == "" {
			return
		}

		root := candidate
		if !filepath.IsAbs(root) {
			root = filepath.Join(rootDir, candidate)
		}
		root = filepath.Clean(root)
		if _, exists := roots[root]; exists {
			return
		}

		_, err := os.Stat(root)
		roots[root] = FileRoot{
			Label:  label,
			Root:   root,
			Exists: err == nil,
		}
	}

	addRoot("Project Workspace", rootDir)
	addRoot("Recordings", filepath.Join(rootDir, "Recordings"))

	for _, streamer := range settings.App.Streamers {
		if streamer == nil {
			continue
		}
		folder := strings.TrimSpace(streamer.Folder)
		if folder == "" {
			continue
		}
		addRoot(fmt.Sprintf("Streamer: %s", streamer.ScreenId), folder)
	}

	list := make([]FileRoot, 0, len(roots))
	for _, root := range roots {
		list = append(list, root)
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].Label == list[j].Label {
			return list[i].Root < list[j].Root
		}
		return list[i].Label < list[j].Label
	})

	return list
}

func validateSchedule(value string) error {
	schedule := strings.TrimSpace(value)
	if schedule == "" {
		return errors.New("schedule is required")
	}
	if strings.HasPrefix(schedule, "@every ") {
		_, err := time.ParseDuration(strings.TrimSpace(strings.TrimPrefix(schedule, "@every ")))
		if err != nil {
			return fmt.Errorf("invalid @every duration: %w", err)
		}
		return nil
	}

	parser := cron.NewParser(
		cron.SecondOptional |
			cron.Minute |
			cron.Hour |
			cron.Dom |
			cron.Month |
			cron.Dow |
			cron.Descriptor,
	)
	_, err := parser.Parse(schedule)
	return err
}
