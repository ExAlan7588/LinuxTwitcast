package discord

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jzhang046/croned-twitcasting-recorder/record"
	"github.com/jzhang046/croned-twitcasting-recorder/twitcasting"
)

var discordAPI = "https://discord.com/api/v10"

const (
	updateInterval = 30 * time.Second
	colorLive      = 0x00a5f9
	colorArchive   = 0x95a5a6
	twitcastIcon   = "https://ja.twitcasting.tv/img/icon192.png"
)

var discordTitleSanitizer = strings.NewReplacer(
	"/", "／",
	"\\", "＼",
	"|", "｜",
	"!", "！",
	"?", "？",
	"*", "＊",
	":", "：",
	"<", "＜",
	">", "＞",
	"\"", "＂",
	"#", "＃",
	"@", "＠",
	"&", "＆",
	"%", "％",
	"$", "＄",
)

var lookupStreamerProfile = twitcasting.LookupStreamerProfile

// Notifier handles Discord notifications for a single recording session.
type Notifier struct {
	botToken         string
	guildID          string
	notifyChannelID  string
	archiveChannelID string
	tagRole          bool
	screenID         string // TwitCasting screen-id used as role name

	roleID    string // lazily resolved/created
	messageID string // ID of the active notify-channel message
	startTime time.Time
	mu        sync.Mutex
	stopChan  chan struct{}

	archiveMessages map[string]archiveMessageState
}

type archiveMessageState struct {
	channelID string
	messageID string
	embed     embed
}

// NewNotifierFromConfig creates a Notifier for the given screen-id.
// Returns nil if notifications are disabled or required fields are missing.
func NewNotifierFromConfig(cfg Config, screenID string) *Notifier {
	if !cfg.Enabled || cfg.BotToken == "" || cfg.NotifyChannelID == "" {
		return nil
	}
	return &Notifier{
		botToken:         cfg.BotToken,
		guildID:          cfg.GuildID,
		notifyChannelID:  cfg.NotifyChannelID,
		archiveChannelID: cfg.ArchiveChannelID,
		tagRole:          cfg.TagRole,
		screenID:         screenID,
		stopChan:         make(chan struct{}),
		archiveMessages:  make(map[string]archiveMessageState),
	}
}

// sanitizeForTitle converts special ASCII characters to full-width equivalents
// so they display cleanly in Discord embed titles.
func sanitizeForTitle(s string) string {
	return discordTitleSanitizer.Replace(s)
}

// FormatTitle returns an embed title in the format [直播主][yyyy-mm-dd]標題.
func FormatTitle(streamerName, title string) string {
	date := time.Now().Format("2006-01-02")
	cleanName := sanitizeForTitle(streamerName)
	cleanTitle := sanitizeForTitle(title)
	if cleanTitle == "" {
		return fmt.Sprintf("[%s][%s]", cleanName, date)
	}
	return fmt.Sprintf("[%s][%s]%s", cleanName, date, cleanTitle)
}

func formatSessionTitle(session record.SessionInfo, texts discordTextCatalog) string {
	titleText := session.Title
	if session.MemberOnly && strings.TrimSpace(titleText) == "" {
		titleText = texts.memberOnlyTitle
	}
	title := FormatTitle(session.StreamerName, titleText)
	if !session.MemberOnly {
		return title
	}
	return texts.memberOnlyTag + title
}

// formatDuration formats a duration as HH:MM:SS.
func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// ── Discord API types ────────────────────────────────────────────────────────

type embed struct {
	Title     string  `json:"title"`
	Url       string  `json:"url,omitempty"`
	Color     int     `json:"color"`
	Author    *author `json:"author,omitempty"`
	Thumbnail *image  `json:"thumbnail,omitempty"`
	Image     *image  `json:"image,omitempty"`
	Fields    []field `json:"fields"`
	Footer    *footer `json:"footer,omitempty"`
	Timestamp string  `json:"timestamp,omitempty"`
}

type author struct {
	Name    string `json:"name"`
	Url     string `json:"url,omitempty"`
	IconUrl string `json:"icon_url,omitempty"`
}

type image struct {
	Url string `json:"url"`
}

type field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type footer struct {
	Text    string `json:"text"`
	IconUrl string `json:"icon_url,omitempty"`
}

// messagePayload supports an optional Content field for role mention text.
type messagePayload struct {
	Content string  `json:"content,omitempty"`
	Embeds  []embed `json:"embeds"`
}

type rolePayload struct {
	Name        string `json:"name"`
	Mentionable bool   `json:"mentionable"`
}

// ── Embed builders ───────────────────────────────────────────────────────────

func defaultDiscordTexts() discordTextCatalog {
	return textsForLanguage(runtimeDiscordConfig(Config{}).EffectiveLanguage())
}

func buildStartEmbedWithTexts(session record.SessionInfo, elapsed time.Duration, texts discordTextCatalog) embed {
	streamURL := buildStreamURL(session.Streamer)
	fields := []field{
		{Name: texts.liveStatusFieldName, Value: texts.liveStatusRecording, Inline: true},
		{Name: texts.recordingDurationField, Value: fmt.Sprintf(texts.recordingDurationUpdating, formatDuration(elapsed)), Inline: true},
	}
	fields = withLiveTypeField(fields, session.MemberOnly, texts)
	fields = withMovieIDField(fields, session.MovieID, texts)
	return embed{
		Title:     formatSessionTitle(session, texts),
		Url:       streamURL,
		Color:     colorLive,
		Author:    buildEmbedAuthor(session, streamURL),
		Thumbnail: buildEmbedThumbnail(session.AvatarURL),
		Image:     buildEmbedImage(session.CoverURL),
		Fields:    fields,
		Footer:    &footer{Text: texts.footerText, IconUrl: twitcastIcon},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

func refreshSessionTitle(session record.SessionInfo) record.SessionInfo {
	profile, err := lookupStreamerProfile(session.Streamer)
	if err != nil || strings.TrimSpace(profile.Title) == "" {
		return session
	}

	refreshed := session
	refreshed.Title = profile.Title
	if strings.TrimSpace(profile.StreamerName) != "" {
		refreshed.StreamerName = profile.StreamerName
	}
	if strings.TrimSpace(profile.AvatarURL) != "" {
		refreshed.AvatarURL = profile.AvatarURL
	}
	return refreshed
}

func buildEndEmbedWithTexts(session record.SessionInfo, elapsed time.Duration, telegramURL string, texts discordTextCatalog) embed {
	streamURL := buildStreamURL(session.Streamer)
	fields := []field{
		{Name: texts.liveStatusFieldName, Value: texts.liveStatusEnded, Inline: true},
		{Name: texts.totalDurationField, Value: fmt.Sprintf("⏱️ **%s**", formatDuration(elapsed)), Inline: true},
	}
	fields = withLiveTypeField(fields, session.MemberOnly, texts)
	fields = withMovieIDField(fields, session.MovieID, texts)
	fields = withTelegramArchiveField(fields, telegramURL, texts)
	return embed{
		Title:     formatSessionTitle(session, texts),
		Url:       streamURL,
		Color:     colorArchive,
		Author:    buildEmbedAuthor(session, streamURL),
		Thumbnail: buildEmbedThumbnail(session.AvatarURL),
		Image:     buildEmbedImage(session.CoverURL),
		Fields:    fields,
		Footer:    &footer{Text: texts.footerText, IconUrl: twitcastIcon},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

func updateLiveSessionFromProfile(session record.SessionInfo) record.SessionInfo {
	refreshed := refreshSessionTitle(session)
	if strings.TrimSpace(refreshed.Title) == "" {
		return session
	}
	return refreshed
}

func buildMemberOnlyStartEmbedWithTexts(session record.SessionInfo, texts discordTextCatalog) embed {
	streamURL := buildStreamURL(session.Streamer)
	fields := []field{
		{Name: texts.liveStatusFieldName, Value: texts.memberOnlyStatusLive, Inline: true},
		{Name: texts.recordingStateField, Value: texts.recordingStateSkipped, Inline: true},
	}
	fields = withMovieIDField(fields, session.MovieID, texts)
	return embed{
		Title:     formatSessionTitle(session, texts),
		Url:       streamURL,
		Color:     colorLive,
		Author:    buildEmbedAuthor(session, streamURL),
		Thumbnail: buildEmbedThumbnail(session.AvatarURL),
		Image:     buildEmbedImage(session.CoverURL),
		Fields:    fields,
		Footer:    &footer{Text: texts.footerText, IconUrl: twitcastIcon},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

func buildMemberOnlyEndEmbedWithTexts(session record.SessionInfo, texts discordTextCatalog) embed {
	streamURL := buildStreamURL(session.Streamer)
	fields := []field{
		{Name: texts.liveStatusFieldName, Value: texts.memberOnlyStatusEnded, Inline: true},
		{Name: texts.recordingStateField, Value: texts.recordingStateNotRecorded, Inline: true},
	}
	fields = withMovieIDField(fields, session.MovieID, texts)
	return embed{
		Title:     formatSessionTitle(session, texts),
		Url:       streamURL,
		Color:     colorArchive,
		Author:    buildEmbedAuthor(session, streamURL),
		Thumbnail: buildEmbedThumbnail(session.AvatarURL),
		Image:     buildEmbedImage(session.CoverURL),
		Fields:    fields,
		Footer:    &footer{Text: texts.footerText, IconUrl: twitcastIcon},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

func buildStreamURL(screenID string) string {
	return "https://twitcasting.tv/" + screenID
}

func buildEmbedAuthor(session record.SessionInfo, streamURL string) *author {
	return &author{
		Name:    formatAuthorName(session),
		Url:     streamURL,
		IconUrl: twitcastIcon,
	}
}

func buildEmbedThumbnail(avatarURL string) *image {
	if strings.TrimSpace(avatarURL) == "" {
		return nil
	}
	return &image{Url: avatarURL}
}

func buildEmbedImage(coverURL string) *image {
	if strings.TrimSpace(coverURL) == "" {
		return nil
	}
	return &image{Url: coverURL}
}

func withTelegramArchiveField(fields []field, telegramURL string, texts discordTextCatalog) []field {
	next := make([]field, 0, len(fields)+1)
	for _, item := range fields {
		if item.Name == texts.archiveFieldName {
			continue
		}
		next = append(next, item)
	}
	if strings.TrimSpace(telegramURL) == "" {
		return next
	}
	next = append(next, field{
		Name:   texts.archiveFieldName,
		Value:  fmt.Sprintf("[%s](%s)", texts.archiveFieldLinkLabel, telegramURL),
		Inline: false,
	})
	return next
}

func withLiveTypeField(fields []field, memberOnly bool, texts discordTextCatalog) []field {
	if !memberOnly {
		return fields
	}
	return append(fields, field{
		Name:   texts.liveTypeFieldName,
		Value:  texts.liveTypeMemberOnly,
		Inline: true,
	})
}

func withMovieIDField(fields []field, movieID string, texts discordTextCatalog) []field {
	trimmedMovieID := strings.TrimSpace(movieID)
	if trimmedMovieID == "" {
		return fields
	}
	return append(fields, field{
		Name:   texts.movieIDFieldName,
		Value:  fmt.Sprintf("🎬 **%s**", trimmedMovieID),
		Inline: true,
	})
}

func formatAuthorName(session record.SessionInfo) string {
	name := strings.TrimSpace(session.StreamerName)
	if name == "" || name == session.Streamer {
		return "@" + session.Streamer
	}
	return fmt.Sprintf("%s (@%s)", name, session.Streamer)
}

func archiveSessionKey(session record.SessionInfo) string {
	return fmt.Sprintf("%s|%d", strings.TrimSpace(session.Filename), session.StartedAt.UnixNano())
}

// ── Notifier internal helpers ────────────────────────────────────────────────

// doRequest delegates to the package-level apiCall helper.
func (n *Notifier) doRequest(method, endpoint string, body interface{}) ([]byte, int, error) {
	return apiCall(n.botToken, method, endpoint, body)
}

// getOrCreateRole resolves the guild role for n.screenID, creating it if needed.
// Delegates to the package-level helper in commands.go.
func (n *Notifier) getOrCreateRole() string {
	return GetOrCreateRoleByScreenID(n.botToken, n.guildID, n.screenID)
}

// roleMentionContent returns "<@&roleID>" when tag_role is enabled,
// or "" when disabled or role resolution fails.
func (n *Notifier) roleMentionContent() string {
	if !n.tagRole || n.guildID == "" {
		return ""
	}
	if n.roleID == "" {
		n.roleID = n.getOrCreateRole()
	}
	if n.roleID == "" {
		return ""
	}
	return fmt.Sprintf("<@&%s>", n.roleID)
}

func (n *Notifier) sendMessageToChannel(channelID, content string, e embed) (string, error) {
	payload := messagePayload{Content: content, Embeds: []embed{e}}
	respBody, status, err := n.doRequest("POST", "/channels/"+channelID+"/messages", payload)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("discord API error %d: %s", status, string(respBody))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}
	if id, ok := result["id"].(string); ok {
		return id, nil
	}
	return "", fmt.Errorf("message ID not found in response")
}

func SendInvalidStreamerIDAlert(cfg Config, screenID string) {
	if !cfg.Enabled || strings.TrimSpace(cfg.BotToken) == "" || strings.TrimSpace(cfg.NotifyChannelID) == "" {
		return
	}

	n := NewNotifierFromConfig(cfg, screenID)
	if n == nil {
		return
	}
	texts := textsForLanguage(cfg.EffectiveLanguage())

	e := embed{
		Title: texts.invalidStreamerTitle,
		Url:   buildStreamURL(screenID),
		Color: 0xe67e22,
		Author: &author{
			Name:    "@" + screenID,
			Url:     buildStreamURL(screenID),
			IconUrl: twitcastIcon,
		},
		Fields: []field{
			{Name: texts.invalidStreamerStatusName, Value: texts.invalidStreamerStatus, Inline: true},
			{Name: "Streamer", Value: "`@" + screenID + "`", Inline: true},
			{Name: texts.invalidStreamerActionName, Value: texts.invalidStreamerAction, Inline: false},
		},
		Footer: &footer{
			Text:    texts.footerText,
			IconUrl: twitcastIcon,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if _, err := n.sendMessageToChannel(cfg.NotifyChannelID, "", e); err != nil {
		log.Printf("[Discord] Failed to send invalid streamer-id alert for [%s]: %v\n", screenID, err)
	}
}

func (n *Notifier) editMessage(channelID, messageID string, e embed) error {
	// PATCH only updates the embed; the original role-mention content is preserved by Discord.
	payload := messagePayload{Embeds: []embed{e}}
	_, status, err := n.doRequest("PATCH", "/channels/"+channelID+"/messages/"+messageID, payload)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("discord PATCH error %d", status)
	}
	return nil
}

func (n *Notifier) deleteMessage(channelID, messageID string) error {
	_, status, err := n.doRequest("DELETE", "/channels/"+channelID+"/messages/"+messageID, nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("discord DELETE error %d", status)
	}
	return nil
}

// ── Public lifecycle methods ─────────────────────────────────────────────────

// NotifyMemberOnlyStart sends a notify-channel message for a members-only live without
// starting the periodic duration updater.
func (n *Notifier) NotifyMemberOnlyStart(session record.SessionInfo) {
	if n == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()

	mention := n.roleMentionContent()
	texts := defaultDiscordTexts()
	e := buildMemberOnlyStartEmbedWithTexts(session, texts)
	msgID, err := n.sendMessageToChannel(n.notifyChannelID, mention, e)
	if err != nil {
		log.Printf("[Discord] Failed to send members-only start notification: %v\n", err)
		return
	}
	n.messageID = msgID

	log.Printf("[Discord] Members-only start notification sent (msgID=%s)\n", msgID)
}

// NotifyMemberOnlyEnd archives the members-only embed and deletes the original notify message.
func (n *Notifier) NotifyMemberOnlyEnd(session record.SessionInfo) {
	if n == nil {
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	texts := defaultDiscordTexts()
	endEmbed := buildMemberOnlyEndEmbedWithTexts(session, texts)
	originalMsgID := n.messageID

	if n.archiveChannelID != "" {
		if _, err := n.sendMessageToChannel(n.archiveChannelID, "", endEmbed); err != nil {
			log.Printf("[Discord] Failed to send members-only archive notification: %v\n", err)
		} else {
			log.Printf("[Discord] Members-only archive notification sent to channel %s\n", n.archiveChannelID)
		}
	}

	if originalMsgID != "" {
		if err := n.deleteMessage(n.notifyChannelID, originalMsgID); err != nil {
			log.Printf("[Discord] Failed to delete original members-only message: %v\n", err)
		} else {
			log.Printf("[Discord] Original members-only message %s deleted\n", originalMsgID)
		}
		n.messageID = ""
	}
}

// DismissMemberOnly deletes the notify-channel members-only message when the
// same streamer becomes recordable before the members-only live ends.
func (n *Notifier) DismissMemberOnly(session record.SessionInfo) {
	if n == nil {
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	originalMsgID := n.messageID
	if originalMsgID == "" {
		return
	}
	if err := n.deleteMessage(n.notifyChannelID, originalMsgID); err != nil {
		log.Printf("[Discord] Failed to dismiss members-only message for [%s]: %v\n", session.Streamer, err)
		return
	}
	n.messageID = ""
	log.Printf("[Discord] Members-only message %s dismissed before recording [%s]\n", originalMsgID, session.Streamer)
}

// NotifyStart sends the initial "recording started" notification and begins
// periodic duration updates.
func (n *Notifier) NotifyStart(session record.SessionInfo) {
	if n == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()

	n.startTime = time.Now()
	mention := n.roleMentionContent()
	texts := defaultDiscordTexts()

	e := buildStartEmbedWithTexts(session, 0, texts)
	msgID, err := n.sendMessageToChannel(n.notifyChannelID, mention, e)
	if err != nil {
		log.Printf("[Discord] Failed to send start notification: %v\n", err)
		return
	}
	n.messageID = msgID

	log.Printf("[Discord] Start notification sent (msgID=%s)\n", msgID)

	// Periodic duration update goroutine
	go func() {
		ticker := time.NewTicker(updateInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n.mu.Lock()
				elapsed := time.Since(n.startTime)
				mID := n.messageID
				currentSession := session
				n.mu.Unlock()

				if mID == "" {
					return
				}
				currentSession = updateLiveSessionFromProfile(currentSession)
				updated := buildStartEmbedWithTexts(currentSession, elapsed, texts)
				if err := n.editMessage(n.notifyChannelID, mID, updated); err != nil {
					log.Printf("[Discord] Failed to update duration: %v\n", err)
				}
			case <-n.stopChan:
				return
			}
		}
	}()
}

// NotifyEnd stops the update loop, archives the ended embed, then deletes the original message.
func (n *Notifier) NotifyEnd(session record.SessionInfo) {
	if n == nil {
		return
	}

	// Signal the updater goroutine to stop
	select {
	case n.stopChan <- struct{}{}:
	default:
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	elapsed := time.Since(n.startTime)
	endEmbed := buildEndEmbedWithTexts(session, elapsed, "", defaultDiscordTexts())
	originalMsgID := n.messageID
	sessionKey := archiveSessionKey(session)

	// 1. Send ended embed to archive channel (no role mention on archive)
	if n.archiveChannelID != "" {
		if archiveMsgID, err := n.sendMessageToChannel(n.archiveChannelID, "", endEmbed); err != nil {
			log.Printf("[Discord] Failed to send archive notification: %v\n", err)
		} else {
			n.archiveMessages[sessionKey] = archiveMessageState{
				channelID: n.archiveChannelID,
				messageID: archiveMsgID,
				embed:     endEmbed,
			}
			log.Printf("[Discord] Archive notification sent to channel %s\n", n.archiveChannelID)
		}
	}

	// 2. Delete the original notify-channel message
	if originalMsgID != "" {
		if err := n.deleteMessage(n.notifyChannelID, originalMsgID); err != nil {
			log.Printf("[Discord] Failed to delete original message: %v\n", err)
		} else {
			log.Printf("[Discord] Original message %s deleted\n", originalMsgID)
		}
		n.messageID = ""
	}
}

// 把 Telegram 归档链接补回同一条归档消息，避免频道里再多发一条通知。
func (n *Notifier) UpdateArchiveWithTelegramLink(session record.SessionInfo, telegramURL string) error {
	if n == nil || strings.TrimSpace(telegramURL) == "" {
		return nil
	}

	sessionKey := archiveSessionKey(session)

	n.mu.Lock()
	state, ok := n.archiveMessages[sessionKey]
	n.mu.Unlock()
	if !ok || state.messageID == "" || state.channelID == "" {
		return nil
	}

	updated := state.embed
	updated.Fields = withTelegramArchiveField(updated.Fields, telegramURL, defaultDiscordTexts())
	if err := n.editMessage(state.channelID, state.messageID, updated); err != nil {
		return err
	}

	n.mu.Lock()
	state.embed = updated
	n.archiveMessages[sessionKey] = state
	n.mu.Unlock()
	return nil
}
