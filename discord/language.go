package discord

import "strings"

const (
	discordLangEnglish = "en"
	discordLangChinese = "zh"
)

type localizedText struct {
	english string
	chinese string
}

type discordTextCatalog struct {
	memberOnlyTag             string
	memberOnlyTitle           string
	footerText                string
	liveStatusFieldName       string
	liveStatusRecording       string
	liveStatusEnded           string
	recordingDurationField    string
	recordingDurationUpdating string
	totalDurationField        string
	memberOnlyStatusLive      string
	memberOnlyStatusEnded     string
	recordingStateField       string
	recordingStateSkipped     string
	recordingStateNotRecorded string
	archiveFieldName          string
	archiveFieldLinkLabel     string
	liveTypeFieldName         string
	liveTypeMemberOnly        string
	movieIDFieldName          string
	invalidStreamerTitle      string
	invalidStreamerStatusName string
	invalidStreamerStatus     string
	invalidStreamerActionName string
	invalidStreamerAction     string
	commandSubscribe          string
	commandUnsubscribe        string
	commandLanguage           string
	commandLanguageOption     string
	commandLanguageDesc       string
	commandLanguageChoiceEN   string
	commandLanguageChoiceZH   string
	onlyInGuild               string
	parseStreamerFailed       string
	roleCreateFailed          string
	subscribeAlready          string
	subscribeAssignFailed     string
	subscribedSuccess         string
	unsubscribeMissing        string
	unsubscribeFailed         string
	unsubscribedSuccess       string
	langUpdatedEN             string
	langUpdatedZH             string
	langInvalid               string
}

func normalizeDiscordLanguage(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case discordLangChinese:
		return discordLangChinese
	default:
		return discordLangEnglish
	}
}

func (cfg Config) EffectiveLanguage() string {
	return normalizeDiscordLanguage(cfg.Language)
}

func (cfg *Config) SetLanguage(lang string) {
	if cfg == nil {
		return
	}
	cfg.Language = normalizeDiscordLanguage(lang)
}

func runtimeDiscordConfig(fallback Config) Config {
	cfg := LoadConfig()
	if strings.TrimSpace(cfg.BotToken) == "" {
		cfg = fallback
	}
	cfg.SetLanguage(cfg.Language)
	return cfg
}

func textsForLanguage(lang string) discordTextCatalog {
	if normalizeDiscordLanguage(lang) == discordLangChinese {
		return discordTextCatalog{
			memberOnlyTag:             "【會員限定】",
			memberOnlyTitle:           "會員限定直播",
			footerText:                "TwitCasting 取流與歸檔系統",
			liveStatusFieldName:       "直播狀態",
			liveStatusRecording:       "🔴 **正在錄影中**",
			liveStatusEnded:           "⏹️ **錄影已結束**",
			recordingDurationField:    "錄影時長",
			recordingDurationUpdating: "⏱️ **%s**（持續更新）",
			totalDurationField:        "總錄影時長",
			memberOnlyStatusLive:      "🔒 **會員限定直播**",
			memberOnlyStatusEnded:     "🗂️ **會員限定直播已結束**",
			recordingStateField:       "錄製狀態",
			recordingStateSkipped:     "🚫 **未嘗試錄製**",
			recordingStateNotRecorded: "🚫 **未錄製**",
			archiveFieldName:          "錄播檔案",
			archiveFieldLinkLabel:     "點我打開 Telegram 錄播檔",
			liveTypeFieldName:         "直播類型",
			liveTypeMemberOnly:        "🔒 **會員限定**",
			movieIDFieldName:          "直播編號",
			invalidStreamerTitle:      "TwitCasting ID已失效",
			invalidStreamerStatusName: "狀態",
			invalidStreamerStatus:     "⚠️ **ID已失效**",
			invalidStreamerActionName: "建議處理",
			invalidStreamerAction:     "請到前端的 General & Streamer Settings 檢查並更新 screen-id。",
			commandSubscribe:          "訂閱直播通知",
			commandUnsubscribe:        "取消訂閱通知",
			commandLanguage:           "lang",
			commandLanguageOption:     "mode",
			commandLanguageDesc:       "切換 Discord 通知語言",
			commandLanguageChoiceEN:   "English",
			commandLanguageChoiceZH:   "繁體中文",
			onlyInGuild:               "❌ 此指令只能在伺服器中使用。",
			parseStreamerFailed:       "❌ 無法從此訊息解析直播主。\n請確認你點的是由 Bot 發出的直播或歸檔通知訊息。",
			roleCreateFailed:          "❌ 無法取得或建立 **@%s** 的身分組。\n請確認 Bot 有「管理身分組」權限，且 Bot 身分組高於目標身分組。",
			subscribeAlready:          "❌ 你已經訂閱過 **@%s** 的通知了！",
			subscribeAssignFailed:     "❌ 無法給予 **@%s** 身分組（HTTP %d）。\n請確認 Bot 身分組順序在目標身分組以上。",
			subscribedSuccess:         "✅ 已給予你 **@%s** 身分組！\n%s 開播時將會收到通知。",
			unsubscribeMissing:        "❌ 你還沒有訂閱 **@%s** 的通知，無法取消！",
			unsubscribeFailed:         "❌ 取消訂閱失敗（HTTP %d）。",
			unsubscribedSuccess:       "❎ 已為你取消 **@%s** 的通知訂閱。",
			langUpdatedEN:             "✅ Discord language switched to **English**.",
			langUpdatedZH:             "✅ Discord 通知語言已切換為 **繁體中文**。",
			langInvalid:               "❌ 不支援的語言。請使用 `en` 或 `zh`。",
		}
	}

	return discordTextCatalog{
		memberOnlyTag:             "[Members Only] ",
		memberOnlyTitle:           "Members-only Live",
		footerText:                "TwitCasting Recorder & Archive",
		liveStatusFieldName:       "Status",
		liveStatusRecording:       "🔴 **Recording in progress**",
		liveStatusEnded:           "⏹️ **Recording finished**",
		recordingDurationField:    "Duration",
		recordingDurationUpdating: "⏱️ **%s** (live updates)",
		totalDurationField:        "Total duration",
		memberOnlyStatusLive:      "🔒 **Members-only live**",
		memberOnlyStatusEnded:     "🗂️ **Members-only live ended**",
		recordingStateField:       "Recorder",
		recordingStateSkipped:     "🚫 **Recording not attempted**",
		recordingStateNotRecorded: "🚫 **Not recorded**",
		archiveFieldName:          "Archive",
		archiveFieldLinkLabel:     "Open Telegram archive",
		liveTypeFieldName:         "Access",
		liveTypeMemberOnly:        "🔒 **Members only**",
		movieIDFieldName:          "Movie ID",
		invalidStreamerTitle:      "TwitCasting ID is no longer valid",
		invalidStreamerStatusName: "Status",
		invalidStreamerStatus:     "⚠️ **ID is invalid**",
		invalidStreamerActionName: "Suggested action",
		invalidStreamerAction:     "Open General & Streamer Settings in the web console and update the screen ID.",
		commandSubscribe:          "Subscribe Stream Alerts",
		commandUnsubscribe:        "Unsubscribe Stream Alerts",
		commandLanguage:           "lang",
		commandLanguageOption:     "mode",
		commandLanguageDesc:       "Switch Discord notification language",
		commandLanguageChoiceEN:   "English",
		commandLanguageChoiceZH:   "繁體中文",
		onlyInGuild:               "❌ This command can only be used inside a server.",
		parseStreamerFailed:       "❌ I couldn't resolve the streamer from this message.\nPlease use it on a live or archive notification posted by the bot.",
		roleCreateFailed:          "❌ Failed to get or create the role for **@%s**.\nMake sure the bot has Manage Roles and its role is above the target role.",
		subscribeAlready:          "❌ You're already subscribed to **@%s**.",
		subscribeAssignFailed:     "❌ Failed to assign **@%s** (HTTP %d).\nMake sure the bot role is above the target role.",
		subscribedSuccess:         "✅ You now have the **@%s** role.\nYou'll be notified when %s goes live.",
		unsubscribeMissing:        "❌ You're not subscribed to **@%s** right now.",
		unsubscribeFailed:         "❌ Failed to unsubscribe (HTTP %d).",
		unsubscribedSuccess:       "❎ You have been unsubscribed from **@%s** alerts.",
		langUpdatedEN:             "✅ Discord notification language is now **English**.",
		langUpdatedZH:             "✅ Discord notification language is now **Traditional Chinese**.",
		langInvalid:               "❌ Unsupported language. Use `en` or `zh`.",
	}
}
