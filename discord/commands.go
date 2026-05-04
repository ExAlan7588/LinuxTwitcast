package discord

import (
	"encoding/json"
	"fmt"
	"log"
)

type discordApplicationUser struct {
	ID string `json:"id"`
}

// FetchAppID retrieves the bot's application ID via GET /users/@me.
// For Discord bots the application ID equals the bot user ID.
func FetchAppID(botToken string) string {
	body, status, err := apiCall(botToken, "GET", "/users/@me", nil)
	if err != nil || status < 200 || status >= 300 {
		log.Printf("[Discord] Failed to fetch application ID (status=%d): %v\n", status, err)
		return ""
	}

	var me discordApplicationUser
	if err := json.Unmarshal(body, &me); err != nil {
		return ""
	}
	if me.ID != "" {
		log.Printf("[Discord] Application ID: %s\n", me.ID)
	}
	return me.ID
}

const (
	contextMenuCommandName      = "Subscribe Stream Alerts"
	contextMenuCommandUnsubName = "Unsubscribe Stream Alerts"
)

// RegisterContextMenuCommand creates (or updates) the message context menu commands
// for the configured guild. Safe to call multiple times.
func RegisterCommands(cfg Config, appID string) {
	if appID == "" || cfg.GuildID == "" {
		log.Println("[Discord] Skipping command registration: appID or GuildID not set")
		return
	}

	texts := textsForLanguage(cfg.EffectiveLanguage())
	commands := []map[string]interface{}{
		{"name": texts.commandSubscribe, "type": messageContextMenuCommandType},
		{"name": texts.commandUnsubscribe, "type": messageContextMenuCommandType},
		{
			"name":        texts.commandLanguage,
			"description": texts.commandLanguageDesc,
			"type":        chatInputCommandType,
			"options": []map[string]interface{}{
				{
					"type":        applicationCommandOptionString,
					"name":        texts.commandLanguageOption,
					"description": texts.commandLanguageDesc,
					"required":    true,
					"choices": []map[string]interface{}{
						{"name": texts.commandLanguageChoiceEN, "value": discordLangEnglish},
						{"name": texts.commandLanguageChoiceZH, "value": discordLangChinese},
					},
				},
			},
		},
	}
	endpoint := fmt.Sprintf("/applications/%s/guilds/%s/commands", appID, cfg.GuildID)
	body, status, err := apiCall(cfg.BotToken, "PUT", endpoint, commands)
	if err != nil || status < 200 || status >= 300 {
		log.Printf("[Discord] Failed to register context menu commands (status=%d): %v %s\n",
			status, err, string(body))
		return
	}
	log.Printf("[Discord] Context menu commands registered/updated\n")
}
