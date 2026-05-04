package discord

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

const (
	applicationCommandInteractionType    = 2
	chatInputCommandType                 = 1
	messageContextMenuCommandType        = 3
	applicationCommandOptionString       = 3
	channelMessageWithSourceResponseType = 4
	deferredChannelMessageWithSourceType = 5
	ephemeralResponseFlag                = 1 << 6
)

type interactionPayload struct {
	ID      string                 `json:"id"`
	Token   string                 `json:"token"`
	Type    int                    `json:"type"`
	GuildID string                 `json:"guild_id"`
	Member  *interactionMember     `json:"member"`
	Data    interactionCommandData `json:"data"`
}

type interactionMember struct {
	User  interactionUser `json:"user"`
	Roles []string        `json:"roles"`
}

type interactionUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type interactionCommandData struct {
	Name     string              `json:"name"`
	Type     int                 `json:"type"`
	TargetID string              `json:"target_id"`
	Options  []interactionOption `json:"options"`
	Resolved interactionResolved `json:"resolved"`
}

type interactionOption struct {
	Name  string `json:"name"`
	Type  int    `json:"type"`
	Value string `json:"value"`
}

type interactionResolved struct {
	Messages map[string]resolvedMessage `json:"messages"`
}

// HandleInteraction processes an INTERACTION_CREATE Gateway event.
func HandleInteraction(cfg Config, appID string, raw json.RawMessage) {
	cfg = runtimeDiscordConfig(cfg)
	ix, err := parseInteractionPayload(raw)
	if err != nil {
		log.Printf("[Discord] Failed to parse interaction: %v\n", err)
		return
	}
	if ix.Member == nil {
		texts := textsForLanguage(cfg.EffectiveLanguage())
		respondEphemeral(cfg.BotToken, ix.ID, ix.Token, texts.onlyInGuild)
		return
	}
	switch {
	case ix.isMessageContextMenuCommand():
		handleContextMenuInteraction(cfg, appID, ix)
	case ix.isLanguageCommand():
		handleLanguageInteraction(cfg, appID, ix)
	}
}

func parseInteractionPayload(raw json.RawMessage) (interactionPayload, error) {
	var ix interactionPayload
	err := json.Unmarshal(raw, &ix)
	return ix, err
}

func (ix interactionPayload) isMessageContextMenuCommand() bool {
	if ix.Type != applicationCommandInteractionType || ix.Data.Type != messageContextMenuCommandType {
		return false
	}
	return ix.Data.Name == contextMenuCommandName || ix.Data.Name == contextMenuCommandUnsubName ||
		ix.Data.Name == textsForLanguage(discordLangChinese).commandSubscribe ||
		ix.Data.Name == textsForLanguage(discordLangChinese).commandUnsubscribe
}

func (ix interactionPayload) isLanguageCommand() bool {
	return ix.Type == applicationCommandInteractionType &&
		ix.Data.Type == chatInputCommandType &&
		ix.Data.Name == textsForLanguage(discordLangEnglish).commandLanguage
}

func handleContextMenuInteraction(cfg Config, appID string, ix interactionPayload) {
	texts := textsForLanguage(cfg.EffectiveLanguage())
	screenID, err := resolveScreenIDFromTargetMessage(ix.Data)
	if err != nil {
		log.Printf("[Discord] Failed to resolve screen ID from target message: %v\n", err)
		respondEphemeral(cfg.BotToken, ix.ID, ix.Token, texts.parseStreamerFailed)
		return
	}

	deferEphemeralResponse(cfg.BotToken, ix.ID, ix.Token)

	guildID := ix.guildID(cfg.GuildID)
	roleID := GetOrCreateRoleByScreenID(cfg.BotToken, guildID, screenID)
	if roleID == "" {
		editInteractionResponse(cfg.BotToken, appID, ix.Token,
			fmt.Sprintf(texts.roleCreateFailed, screenID))
		return
	}

	if isSubscribeCommandName(ix.Data.Name) {
		subscribeMember(cfg, appID, ix, guildID, roleID, screenID, texts)
		return
	}

	unsubscribeMember(cfg, appID, ix, guildID, roleID, screenID, texts)
}

func (ix interactionPayload) guildID(defaultGuildID string) string {
	if ix.GuildID != "" {
		return ix.GuildID
	}
	return defaultGuildID
}

func subscribeMember(cfg Config, appID string, ix interactionPayload, guildID, roleID, screenID string, texts discordTextCatalog) {
	if memberHasRole(ix.Member, roleID) {
		editInteractionResponse(cfg.BotToken, appID, ix.Token, fmt.Sprintf(texts.subscribeAlready, screenID))
		return
	}

	if err := assignRoleToMember(cfg.BotToken, guildID, ix.Member.User.ID, roleID); err != nil {
		log.Printf("[Discord] Failed to assign role '%s' to user %s (status=%d): %v\n",
			screenID, ix.Member.User.ID, err.status, err.err)
		editInteractionResponse(cfg.BotToken, appID, ix.Token,
			fmt.Sprintf(texts.subscribeAssignFailed, screenID, err.status))
		return
	}

	log.Printf("[Discord] Assigned role '%s' to user %s (%s)\n", screenID, ix.Member.User.Username, ix.Member.User.ID)
	editInteractionResponse(cfg.BotToken, appID, ix.Token,
		fmt.Sprintf(texts.subscribedSuccess, screenID, screenID))
}

func unsubscribeMember(cfg Config, appID string, ix interactionPayload, guildID, roleID, screenID string, texts discordTextCatalog) {
	if !memberHasRole(ix.Member, roleID) {
		editInteractionResponse(cfg.BotToken, appID, ix.Token, fmt.Sprintf(texts.unsubscribeMissing, screenID))
		return
	}

	if err := removeRoleFromMember(cfg.BotToken, guildID, ix.Member.User.ID, roleID); err != nil {
		log.Printf("[Discord] Failed to remove role '%s' from user %s (status=%d): %v\n",
			screenID, ix.Member.User.ID, err.status, err.err)
		editInteractionResponse(cfg.BotToken, appID, ix.Token,
			fmt.Sprintf(texts.unsubscribeFailed, err.status))
		return
	}

	log.Printf("[Discord] Removed role '%s' from user %s (%s)\n", screenID, ix.Member.User.Username, ix.Member.User.ID)
	editInteractionResponse(cfg.BotToken, appID, ix.Token, fmt.Sprintf(texts.unsubscribedSuccess, screenID))
}

func handleLanguageInteraction(cfg Config, appID string, ix interactionPayload) {
	deferEphemeralResponse(cfg.BotToken, ix.ID, ix.Token)

	rawSelected := strings.ToLower(strings.TrimSpace(ix.Data.optionValue(textsForLanguage(discordLangEnglish).commandLanguageOption)))
	if rawSelected != discordLangEnglish && rawSelected != discordLangChinese {
		editInteractionResponse(cfg.BotToken, appID, ix.Token, textsForLanguage(cfg.EffectiveLanguage()).langInvalid)
		return
	}
	selected := normalizeDiscordLanguage(rawSelected)

	current := LoadConfig()
	if strings.TrimSpace(current.BotToken) == "" {
		current = cfg
	}
	current.SetLanguage(selected)
	if err := SaveConfig(current); err != nil {
		log.Printf("[Discord] Failed to save language config: %v\n", err)
		editInteractionResponse(cfg.BotToken, appID, ix.Token, fmt.Sprintf("❌ Failed to save Discord language: %v", err))
		return
	}
	RegisterCommands(current, appID)

	texts := textsForLanguage(selected)
	if selected == discordLangChinese {
		editInteractionResponse(cfg.BotToken, appID, ix.Token, texts.langUpdatedZH)
		return
	}
	editInteractionResponse(cfg.BotToken, appID, ix.Token, texts.langUpdatedEN)
}

func isSubscribeCommandName(name string) bool {
	return name == textsForLanguage(discordLangEnglish).commandSubscribe ||
		name == textsForLanguage(discordLangChinese).commandSubscribe
}

func (data interactionCommandData) optionValue(name string) string {
	for _, option := range data.Options {
		if option.Name == name {
			return option.Value
		}
	}
	return ""
}

func memberHasRole(member *interactionMember, roleID string) bool {
	if member == nil {
		return false
	}
	for _, currentRoleID := range member.Roles {
		if currentRoleID == roleID {
			return true
		}
	}
	return false
}

type roleMutationError struct {
	status int
	err    error
}

func assignRoleToMember(botToken, guildID, userID, roleID string) *roleMutationError {
	return mutateMemberRole(botToken, "PUT", guildID, userID, roleID)
}

func removeRoleFromMember(botToken, guildID, userID, roleID string) *roleMutationError {
	return mutateMemberRole(botToken, "DELETE", guildID, userID, roleID)
}

func mutateMemberRole(botToken, method, guildID, userID, roleID string) *roleMutationError {
	endpoint := fmt.Sprintf("/guilds/%s/members/%s/roles/%s", guildID, userID, roleID)
	body, status, err := apiCall(botToken, method, endpoint, nil)
	if err == nil && status >= 200 && status < 300 {
		return nil
	}
	if err == nil {
		err = fmt.Errorf("discord API error %d: %s", status, string(body))
	}
	return &roleMutationError{status: status, err: err}
}

// respondEphemeral sends an immediate ephemeral response to an interaction.
func respondEphemeral(botToken, interactionID, token, content string) {
	endpoint := fmt.Sprintf("/interactions/%s/%s/callback", interactionID, token)
	payload := map[string]interface{}{
		"type": channelMessageWithSourceResponseType,
		"data": map[string]interface{}{
			"content": content,
			"flags":   ephemeralResponseFlag,
		},
	}
	apiCall(botToken, "POST", endpoint, payload) //nolint:errcheck
}

// deferEphemeralResponse sends a deferred "thinking..." ephemeral response.
// Call editInteractionResponse within 15 minutes to fill it in.
func deferEphemeralResponse(botToken, interactionID, token string) {
	endpoint := fmt.Sprintf("/interactions/%s/%s/callback", interactionID, token)
	payload := map[string]interface{}{
		"type": deferredChannelMessageWithSourceType,
		"data": map[string]interface{}{
			"flags": ephemeralResponseFlag,
		},
	}
	apiCall(botToken, "POST", endpoint, payload) //nolint:errcheck
}

// editInteractionResponse edits the deferred interaction response.
func editInteractionResponse(botToken, appID, token, content string) {
	endpoint := fmt.Sprintf("/webhooks/%s/%s/messages/@original", appID, token)
	payload := map[string]interface{}{"content": content}
	apiCall(botToken, "PATCH", endpoint, payload) //nolint:errcheck
}
