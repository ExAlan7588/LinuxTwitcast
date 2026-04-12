package discord

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
)

// ── Message-to-screenID mapping ──────────────────────────────────────────────

var (
	msgMapMu           sync.RWMutex
	messageScreenIDMap = make(map[string]string) // Discord msgID -> TwitCasting screenID
)

// AddMessageMapping records which streamer a Discord notification message belongs to.
func AddMessageMapping(messageID, screenID string) {
	if messageID == "" || screenID == "" {
		return
	}
	msgMapMu.Lock()
	messageScreenIDMap[messageID] = screenID
	msgMapMu.Unlock()
	log.Printf("[MsgMap] Tracking message %s -> streamer %s\n", messageID, screenID)
}

// RemoveMessageMapping removes the mapping when a notification message is deleted.
func RemoveMessageMapping(messageID string) {
	if messageID == "" {
		return
	}
	msgMapMu.Lock()
	delete(messageScreenIDMap, messageID)
	msgMapMu.Unlock()
}

// getScreenIDForMessage looks up the TwitCasting screen-id for a Discord message.
func getScreenIDForMessage(messageID string) (string, bool) {
	msgMapMu.RLock()
	defer msgMapMu.RUnlock()
	v, ok := messageScreenIDMap[messageID]
	return v, ok
}

// ── Application ID ───────────────────────────────────────────────────────────

// FetchAppID retrieves the bot's application ID via GET /users/@me.
// For Discord bots the application ID equals the bot user ID.
func FetchAppID(botToken string) string {
	body, status, err := apiCall(botToken, "GET", "/users/@me", nil)
	if err != nil || status < 200 || status >= 300 {
		log.Printf("[Discord] Failed to fetch application ID (status=%d): %v\n", status, err)
		return ""
	}
	var me map[string]interface{}
	if err := json.Unmarshal(body, &me); err != nil {
		return ""
	}
	if id, ok := me["id"].(string); ok {
		log.Printf("[Discord] Application ID: %s\n", id)
		return id
	}
	return ""
}

// ── Context menu command registration ───────────────────────────────────────

const (
	contextMenuCommandName      = "訂閱直播通知"
	contextMenuCommandUnsubName = "取消訂閱通知"
)

// RegisterContextMenuCommand creates (or updates) the message context menu commands
// for the configured guild. Safe to call multiple times.
func RegisterContextMenuCommand(cfg Config, appID string) {
	if appID == "" || cfg.GuildID == "" {
		log.Println("[Discord] Skipping command registration: appID or GuildID not set")
		return
	}
	// PUT to overwrite all guild commands with these two
	commands := []map[string]interface{}{
		{
			"name": contextMenuCommandName,
			"type": 3, // MESSAGE context menu
		},
		{
			"name": contextMenuCommandUnsubName,
			"type": 3,
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

// ── Shared role helper ───────────────────────────────────────────────────────

// GetOrCreateRoleByScreenID looks up a guild role named screenID and creates it
// (mentionable) if it does not exist. Returns "" on failure.
func GetOrCreateRoleByScreenID(botToken, guildID, screenID string) string {
	if botToken == "" || guildID == "" || screenID == "" {
		return ""
	}

	// Fetch existing roles
	body, status, err := apiCall(botToken, "GET", "/guilds/"+guildID+"/roles", nil)
	if err != nil || status < 200 || status >= 300 {
		log.Printf("[Discord] Failed to fetch guild roles (status=%d): %v\n", status, err)
		return ""
	}
	var roles []map[string]interface{}
	if err := json.Unmarshal(body, &roles); err != nil {
		return ""
	}
	for _, r := range roles {
		if name, ok := r["name"].(string); ok && name == screenID {
			if id, ok := r["id"].(string); ok {
				return id
			}
		}
	}

	// Role not found — create it
	respBody, status, err := apiCall(botToken, "POST", "/guilds/"+guildID+"/roles",
		rolePayload{Name: screenID, Mentionable: true})
	if err != nil || status < 200 || status >= 300 {
		log.Printf("[Discord] Failed to create role '%s' (status=%d): %v\n", screenID, status, err)
		return ""
	}
	var created map[string]interface{}
	if err := json.Unmarshal(respBody, &created); err != nil {
		return ""
	}
	if id, ok := created["id"].(string); ok {
		log.Printf("[Discord] Created role '%s' (id=%s)\n", screenID, id)
		return id
	}
	return ""
}

// ── Interaction handling ─────────────────────────────────────────────────────

// interactionPayload holds the fields we need from INTERACTION_CREATE.
type interactionPayload struct {
	ID          string `json:"id"`
	Token       string `json:"token"`
	Type        int    `json:"type"`
	GuildID     string `json:"guild_id"`
	Member      *struct {
		User struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		} `json:"user"`
		Roles []string `json:"roles"`
	} `json:"member"`
	Data struct {
		Name     string `json:"name"`
		Type     int    `json:"type"`
		TargetID string `json:"target_id"` // right-clicked message ID
	} `json:"data"`
}

// HandleInteraction processes an INTERACTION_CREATE Gateway event.
func HandleInteraction(cfg Config, appID string, raw json.RawMessage) {
	var ix interactionPayload
	if err := json.Unmarshal(raw, &ix); err != nil {
		log.Printf("[Discord] Failed to parse interaction: %v\n", err)
		return
	}

	// Only handle Application Command (type 2) + Message context menu (data.type 3)
	if ix.Type != 2 || ix.Data.Type != 3 {
		return
	}
	if ix.Data.Name != contextMenuCommandName && ix.Data.Name != contextMenuCommandUnsubName {
		return
	}

	// Safety: need member info
	if ix.Member == nil {
		respondEphemeral(cfg.BotToken, ix.ID, ix.Token,
			"❌ 此指令只能在伺服器中使用。")
		return
	}

	targetMsgID := ix.Data.TargetID
	screenID, ok := getScreenIDForMessage(targetMsgID)
	if !ok {
		respondEphemeral(cfg.BotToken, ix.ID, ix.Token,
			"❌ 找不到此訊息對應的直播主。\n可能是舊訊息或程式重啟後遺失的記錄，請等待下次開播時重試。")
		return
	}

	guildID := ix.GuildID
	if guildID == "" {
		guildID = cfg.GuildID
	}
	userID := ix.Member.User.ID

	// Defer response so we have time to do the role operations
	deferEphemeralResponse(cfg.BotToken, ix.ID, ix.Token)

	// Resolve or create the role
	roleID := GetOrCreateRoleByScreenID(cfg.BotToken, guildID, screenID)
	if roleID == "" {
		editInteractionResponse(cfg.BotToken, appID, ix.Token,
			fmt.Sprintf("❌ 無法取得或建立 **%s** 的身分組。\n請確認 Bot 有「管理身分組」權限，且 Bot 身分組高於目標身分組。", screenID))
		return
	}

	// Check if user already has the role
	hasRole := false
	for _, r := range ix.Member.Roles {
		if r == roleID {
			hasRole = true
			break
		}
	}

	isSubscribing := (ix.Data.Name == contextMenuCommandName)

	if isSubscribing {
		if hasRole {
			editInteractionResponse(cfg.BotToken, appID, ix.Token,
				fmt.Sprintf("❌ 你已經訂閱過 **@%s** 的通知了！", screenID))
			return
		}

		// Assign role
		assignEndpoint := fmt.Sprintf("/guilds/%s/members/%s/roles/%s", guildID, userID, roleID)
		_, status, err := apiCall(cfg.BotToken, "PUT", assignEndpoint, nil)
		if err != nil || status < 200 || status >= 300 {
			log.Printf("[Discord] Failed to assign role '%s' to user %s (status=%d): %v\n",
				screenID, userID, status, err)
			editInteractionResponse(cfg.BotToken, appID, ix.Token,
				fmt.Sprintf("❌ 無法給予 **@%s** 身分組（HTTP %d）。\n請確認 Bot 身分組順序在目標身分組以上。", screenID, status))
			return
		}

		log.Printf("[Discord] Assigned role '%s' to user %s (%s)\n", screenID, ix.Member.User.Username, userID)
		editInteractionResponse(cfg.BotToken, appID, ix.Token,
			fmt.Sprintf("✅ 已給予你 **@%s** 身分組！\n%s 開播時將會收到通知。", screenID, screenID))

	} else {
		// Unsubscribe
		if !hasRole {
			editInteractionResponse(cfg.BotToken, appID, ix.Token,
				fmt.Sprintf("❌ 你還沒有訂閱 **@%s** 的通知，無法取消！", screenID))
			return
		}

		// Remove role
		removeEndpoint := fmt.Sprintf("/guilds/%s/members/%s/roles/%s", guildID, userID, roleID)
		_, status, err := apiCall(cfg.BotToken, "DELETE", removeEndpoint, nil)
		if err != nil || status < 200 || status >= 300 {
			log.Printf("[Discord] Failed to remove role '%s' from user %s (status=%d): %v\n",
				screenID, userID, status, err)
			editInteractionResponse(cfg.BotToken, appID, ix.Token,
				fmt.Sprintf("❌ 取消訂閱失敗（HTTP %d）。", status))
			return
		}

		log.Printf("[Discord] Removed role '%s' from user %s (%s)\n", screenID, ix.Member.User.Username, userID)
		editInteractionResponse(cfg.BotToken, appID, ix.Token,
			fmt.Sprintf("❎ 已為你取消 **@%s** 的通知訂閱。", screenID))
	}
}

// ── Interaction response helpers ─────────────────────────────────────────────

// respondEphemeral sends an immediate ephemeral response to an interaction.
func respondEphemeral(botToken, interactionID, token, content string) {
	endpoint := fmt.Sprintf("/interactions/%s/%s/callback", interactionID, token)
	payload := map[string]interface{}{
		"type": 4, // CHANNEL_MESSAGE_WITH_SOURCE
		"data": map[string]interface{}{
			"content": content,
			"flags":   64, // EPHEMERAL
		},
	}
	apiCall(botToken, "POST", endpoint, payload) //nolint:errcheck
}

// deferEphemeralResponse sends a deferred "thinking..." ephemeral response.
// Call editInteractionResponse within 15 minutes to fill it in.
func deferEphemeralResponse(botToken, interactionID, token string) {
	endpoint := fmt.Sprintf("/interactions/%s/%s/callback", interactionID, token)
	payload := map[string]interface{}{
		"type": 5, // DEFERRED_CHANNEL_MESSAGE_WITH_SOURCE
		"data": map[string]interface{}{
			"flags": 64, // EPHEMERAL
		},
	}
	apiCall(botToken, "POST", endpoint, payload) //nolint:errcheck
}

// editInteractionResponse edits the deferred interaction response.
func editInteractionResponse(botToken, appID, token, content string) {
	endpoint := fmt.Sprintf("/webhooks/%s/%s/messages/@original", appID, token)
	payload := map[string]interface{}{
		"content": content,
	}
	apiCall(botToken, "PATCH", endpoint, payload) //nolint:errcheck
}
