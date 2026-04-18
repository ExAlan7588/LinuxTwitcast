package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/jzhang046/croned-twitcasting-recorder/applog"
	"github.com/jzhang046/croned-twitcasting-recorder/config"
	"github.com/jzhang046/croned-twitcasting-recorder/discord"
	"github.com/jzhang046/croned-twitcasting-recorder/record"
	"github.com/jzhang046/croned-twitcasting-recorder/sink"
	"github.com/jzhang046/croned-twitcasting-recorder/state"
	"github.com/jzhang046/croned-twitcasting-recorder/telegram"
	"github.com/jzhang046/croned-twitcasting-recorder/twitcasting"
)

type Status struct {
	Running          bool                 `json:"running"`
	Stopping         bool                 `json:"stopping"`
	StartedAt        time.Time            `json:"started_at,omitempty"`
	Uptime           string               `json:"uptime,omitempty"`
	TotalStreamers   int                  `json:"total_streamers"`
	EnabledStreamers int                  `json:"enabled_streamers"`
	ScheduledJobs    int                  `json:"scheduled_jobs"`
	ActiveRecordings []record.SessionInfo `json:"active_recordings"`
	Warnings         []Warning            `json:"warnings,omitempty"`
	LastError        string               `json:"last_error,omitempty"`
}

type Warning struct {
	Code     string    `json:"code"`
	Streamer string    `json:"streamer"`
	RetryAt  time.Time `json:"retry_at,omitempty"`
}

type ManualRecordResult struct {
	Streamer     string `json:"streamer"`
	StreamerName string `json:"streamer_name"`
	Title        string `json:"title"`
	MovieID      string `json:"movie_id,omitempty"`
	Folder       string `json:"folder"`
}

type streamWarning struct {
	Code    string
	RetryAt time.Time
}

type memberOnlyNotification struct {
	session  record.SessionInfo
	notifier *discord.Notifier
}

type Manager struct {
	mu sync.RWMutex

	running          bool
	stopping         bool
	startedAt        time.Time
	cancel           context.CancelFunc
	cron             *cron.Cron
	totalStreamers   int
	enabledStreamers int
	scheduledJobs    int
	active           map[string]record.SessionInfo
	warnings         map[string]streamWarning
	memberOnly       map[string]memberOnlyNotification
	discordCfg       discord.Config
	lastError        string
}

const passwordRetryCooldown = 2 * time.Minute

func NewManager() *Manager {
	return &Manager{
		active:     make(map[string]record.SessionInfo),
		warnings:   make(map[string]streamWarning),
		memberOnly: make(map[string]memberOnlyNotification),
	}
}

func (m *Manager) Start() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return errors.New("recorder is already running")
	}
	m.stopping = false
	m.mu.Unlock()

	state.ClearAll()

	cfg, err := config.LoadDefault()
	if err != nil {
		m.storeError(err)
		return err
	}
	enabledStreamers := config.EnabledStreamers(cfg)
	if enabledStreamers == 0 {
		err := errors.New("no enabled streamers configured")
		m.storeError(err)
		return err
	}

	if err := applog.Configure(cfg.EnableLog); err != nil {
		log.Printf("Failed enabling app.log output: %v\n", err)
		_ = applog.Configure(false)
	}

	rootCtx, cancel := context.WithCancel(context.Background())
	scheduler := cron.New(cron.WithChain(
		cron.Recover(cron.DefaultLogger),
		cron.SkipIfStillRunning(cron.DefaultLogger),
	))

	telegramCfg := telegram.LoadConfig()
	discordCfg := discord.LoadConfig()

	var appID string
	if discordCfg.Enabled && discordCfg.BotToken != "" {
		log.Printf("[Discord] Notifications enabled (guild=%s, notify=%s, archive=%s, tagRole=%v)\n",
			discordCfg.GuildID, discordCfg.NotifyChannelID, discordCfg.ArchiveChannelID, discordCfg.TagRole)
		appID = discord.FetchAppID(discordCfg.BotToken)
	} else {
		log.Println("[Discord] Notifications disabled")
	}

	scheduledJobs := 0
	for _, streamerCfg := range cfg.Streamers {
		if streamerCfg == nil || !streamerCfg.Enabled {
			if streamerCfg != nil {
				log.Printf("Skipping disabled streamer [%s]\n", streamerCfg.ScreenId)
			}
			continue
		}
		streamerCfg := *streamerCfg

		recordFunc := m.newRecordFunc(
			rootCtx,
			streamerCfg,
			telegramCfg,
			discordCfg,
			func(streamer string) (record.StreamLookupResult, error) {
				// 直播密码按每个 streamer 独立传入；没填时会在抓流阶段返回专门的缺密码错误。
				return twitcasting.GetWSStreamUrlWithPassword(streamer, streamerCfg.Password)
			},
		)

		_, err := scheduler.AddFunc(
			streamerCfg.Schedule,
			func() {
				if m.shouldSkipStreamer(streamerCfg.ScreenId) {
					return
				}
				recordFunc()
			},
		)
		if err != nil {
			cancel()
			m.storeError(err)
			return fmt.Errorf("failed adding schedule for [%s]: %w", streamerCfg.ScreenId, err)
		}

		scheduledJobs++
		log.Printf("Added schedule [%s] for streamer [%s]\n", streamerCfg.Schedule, streamerCfg.ScreenId)
	}

	if discordCfg.Enabled && discordCfg.BotToken != "" && appID != "" {
		gateway := discord.NewGateway(discordCfg, appID)
		go gateway.Run(rootCtx)
		log.Println("[Discord] Gateway started")
	}

	m.mu.Lock()
	m.running = true
	m.cancel = cancel
	m.cron = scheduler
	m.startedAt = time.Now()
	m.totalStreamers = len(cfg.Streamers)
	m.enabledStreamers = enabledStreamers
	m.scheduledJobs = scheduledJobs
	m.active = make(map[string]record.SessionInfo)
	m.warnings = make(map[string]streamWarning)
	m.memberOnly = make(map[string]memberOnlyNotification)
	m.discordCfg = discordCfg
	m.lastError = ""
	m.mu.Unlock()

	scheduler.Start()
	log.Println("Croned recorder started")
	return nil
}

func (m *Manager) StartManualRecording(rawURL string) (ManualRecordResult, error) {
	cfg, err := config.LoadDefault()
	if err != nil {
		m.storeError(err)
		return ManualRecordResult{}, err
	}

	target, err := twitcasting.ParseManualRecordTarget(rawURL)
	if err != nil {
		m.storeError(err)
		return ManualRecordResult{}, err
	}
	if m.isStreamerActive(target.ScreenID) {
		err = fmt.Errorf("streamer [%s] is already recording", target.ScreenID)
		m.storeError(err)
		return ManualRecordResult{}, err
	}

	streamerCfg := resolveManualStreamerConfig(cfg, target.ScreenID)
	telegramCfg := telegram.LoadConfig()
	discordCfg := discord.LoadConfig()

	target, lookup, err := twitcasting.LookupManualRecordingTarget(rawURL, streamerCfg.Password)
	m.handleStreamLookup(target.ScreenID, err)
	if err != nil {
		m.storeError(err)
		return ManualRecordResult{
			Streamer:     target.ScreenID,
			StreamerName: lookup.StreamerName,
			Title:        lookup.Title,
			MovieID:      lookup.MovieID,
			Folder:       streamerCfg.Folder,
		}, err
	}

	recordFunc := m.newRecordFunc(
		context.Background(),
		streamerCfg,
		telegramCfg,
		discordCfg,
		func(streamer string) (record.StreamLookupResult, error) {
			if strings.TrimSpace(streamer) != strings.TrimSpace(target.ScreenID) {
				return record.StreamLookupResult{}, fmt.Errorf("manual record streamer mismatch: %s", streamer)
			}
			return lookup, nil
		},
	)
	go recordFunc()

	return ManualRecordResult{
		Streamer:     target.ScreenID,
		StreamerName: lookup.StreamerName,
		Title:        lookup.Title,
		MovieID:      lookup.MovieID,
		Folder:       streamerCfg.Folder,
	}, nil
}

func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	if !m.running {
		m.stopping = false
		m.mu.Unlock()
		return nil
	}

	cancel := m.cancel
	scheduler := m.cron
	m.stopping = true
	m.mu.Unlock()

	log.Println("Stopping croned recorder")
	if cancel != nil {
		cancel()
	}

	var err error
	if scheduler != nil {
		done := scheduler.Stop()
		select {
		case <-done.Done():
		case <-ctx.Done():
			err = ctx.Err()
		}
	}

	log.Println("Cron jobs stopped. Waiting for background processors (Telegram/Discord) to finish...")

	// Wait for any background post-processors (like FFmpeg to push Telegram)
	doneWaiting := make(chan struct{})
	go func() {
		record.BackgroundProcessorWg.Wait()
		close(doneWaiting)
	}()

	select {
	case <-doneWaiting:
		log.Println("All background processors finished cleanly.")
	case <-ctx.Done():
		log.Println("Timeout reached while waiting for background processors.")
		err = ctx.Err()
	}

	m.mu.Lock()
	m.running = false
	m.stopping = false
	m.cancel = nil
	m.cron = nil
	m.startedAt = time.Time{}
	m.totalStreamers = 0
	m.enabledStreamers = 0
	m.scheduledJobs = 0
	m.active = make(map[string]record.SessionInfo)
	m.warnings = make(map[string]streamWarning)
	m.memberOnly = make(map[string]memberOnlyNotification)
	m.discordCfg = discord.Config{}
	if err != nil {
		m.lastError = err.Error()
	}
	m.mu.Unlock()

	return err
}

func (m *Manager) Restart(ctx context.Context) error {
	if err := m.Stop(ctx); err != nil {
		return err
	}
	return m.Start()
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	active := make([]record.SessionInfo, 0, len(m.active))
	for _, session := range m.active {
		active = append(active, session)
	}
	sort.Slice(active, func(i, j int) bool {
		return active[i].StartedAt.Before(active[j].StartedAt)
	})

	warnings := make([]Warning, 0, len(m.warnings))
	for streamer, warning := range m.warnings {
		warnings = append(warnings, Warning{
			Code:     warning.Code,
			Streamer: streamer,
			RetryAt:  warning.RetryAt,
		})
	}
	sort.Slice(warnings, func(i, j int) bool {
		return warnings[i].Streamer < warnings[j].Streamer
	})

	status := Status{
		Running:          m.running,
		Stopping:         m.stopping,
		StartedAt:        m.startedAt,
		TotalStreamers:   m.totalStreamers,
		EnabledStreamers: m.enabledStreamers,
		ScheduledJobs:    m.scheduledJobs,
		ActiveRecordings: active,
		Warnings:         warnings,
		LastError:        m.lastError,
	}
	if m.running && !m.startedAt.IsZero() {
		status.Uptime = time.Since(m.startedAt).Round(time.Second).String()
	}

	return status
}

func (m *Manager) handleSessionStart(session record.SessionInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active[session.Streamer] = session
	delete(m.warnings, session.Streamer)
}

func (m *Manager) newRecordFunc(
	rootCtx context.Context,
	streamerCfg config.StreamerConfig,
	telegramCfg telegram.Config,
	discordCfg discord.Config,
	streamURLFetcher func(string) (record.StreamLookupResult, error),
) func() {
	var streamerNotifier record.DiscordNotifier
	var discordNotifier *discord.Notifier
	if notifier := discord.NewNotifierFromConfig(discordCfg, streamerCfg.ScreenId); notifier != nil {
		streamerNotifier = notifier
		discordNotifier = notifier
	}

	return record.ToRecordFunc(&record.RecordConfig{
		Streamer:         streamerCfg.ScreenId,
		Folder:           streamerCfg.Folder,
		Password:         streamerCfg.Password,
		StreamUrlFetcher: streamURLFetcher,
		SinkProvider:     sink.NewFileSink,
		StreamRecorder:   twitcasting.RecordWS,
		RootContext:      rootCtx,
		Notifier:         streamerNotifier,
		PostProcessor: func(session record.SessionInfo) {
			uploadResult := telegram.Process(telegramCfg, session)
			if discordNotifier != nil && strings.TrimSpace(uploadResult.MessageURL) != "" {
				if err := discordNotifier.UpdateArchiveWithTelegramLink(session, uploadResult.MessageURL); err != nil {
					log.Printf("[Discord] Failed to attach Telegram archive link for [%s]: %v\n", session.Streamer, err)
				}
			}
		},
		OnSessionStart: m.handleSessionStart,
		OnSessionEnd:   m.handleSessionEnd,
		OnStreamLookup: m.handleStreamLookup,
	})
}

func (m *Manager) handleSessionEnd(session record.SessionInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.active, session.Streamer)
}

func (m *Manager) isStreamerActive(streamer string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.active[streamer]
	return exists
}

func (m *Manager) storeError(err error) {
	if err == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastError = err.Error()
}

func (m *Manager) shouldSkipStreamer(streamer string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.active[streamer]; ok {
		return true
	}
	if warning, ok := m.warnings[streamer]; ok && warning.Code == "stream_password_required" && !warning.RetryAt.IsZero() {
		return time.Now().Before(warning.RetryAt)
	}
	return false
}

// 密码锁页会让高频排程不断重试；记录一个短冷却，避免每 5 秒都重复打目标页和刷日志。
func (m *Manager) handleStreamLookup(streamer string, err error) {
	var shouldNotifyInvalid bool
	var startMemberOnly *memberOnlyNotification
	var endMemberOnly *memberOnlyNotification
	var discordCfg discord.Config

	lookup, _ := twitcasting.LookupResultFromError(err)

	m.mu.Lock()
	discordCfg = m.discordCfg
	if existing, ok := m.memberOnly[streamer]; ok && !errors.Is(err, twitcasting.ErrMemberOnlyLive) {
		delete(m.memberOnly, streamer)
		copy := existing
		endMemberOnly = &copy
	}

	switch {
	case err == nil:
		delete(m.warnings, streamer)

	case errors.Is(err, twitcasting.ErrPasswordRequired):
		m.warnings[streamer] = streamWarning{
			Code:    "stream_password_required",
			RetryAt: time.Now().Add(passwordRetryCooldown),
		}

	case errors.Is(err, twitcasting.ErrMemberOnlyLive):
		m.warnings[streamer] = streamWarning{Code: "stream_member_only"}
		if _, exists := m.memberOnly[streamer]; !exists {
			streamerName := strings.TrimSpace(lookup.StreamerName)
			if streamerName == "" {
				streamerName = streamer
			}
			entry := memberOnlyNotification{
				session: record.SessionInfo{
					Streamer:     streamer,
					StreamerName: streamerName,
					Title:        lookup.Title,
					AvatarURL:    lookup.AvatarURL,
					CoverURL:     lookup.CoverURL,
					StartedAt:    time.Now(),
				},
				notifier: discord.NewNotifierFromConfig(discordCfg, streamer),
			}
			m.memberOnly[streamer] = entry
			copy := entry
			startMemberOnly = &copy
		}

	case errors.Is(err, twitcasting.ErrStreamerNotFound):
		prev, existed := m.warnings[streamer]
		m.warnings[streamer] = streamWarning{Code: "streamer_id_invalid"}
		if !existed || prev.Code != "streamer_id_invalid" {
			shouldNotifyInvalid = true
		}

	case errors.Is(err, twitcasting.ErrStreamOffline):
		delete(m.warnings, streamer)
	}
	m.mu.Unlock()

	if startMemberOnly != nil && startMemberOnly.notifier != nil {
		go startMemberOnly.notifier.NotifyMemberOnlyStart(startMemberOnly.session)
	}
	if endMemberOnly != nil && endMemberOnly.notifier != nil {
		go endMemberOnly.notifier.NotifyMemberOnlyEnd(endMemberOnly.session)
	}
	if shouldNotifyInvalid {
		go discord.SendInvalidStreamerIDAlert(discordCfg, streamer)
	}
}

func resolveManualStreamerConfig(cfg *config.AppConfig, screenID string) config.StreamerConfig {
	trimmedScreenID := strings.TrimSpace(screenID)
	if cfg != nil {
		for _, streamerCfg := range cfg.Streamers {
			if streamerCfg == nil {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(streamerCfg.ScreenId), trimmedScreenID) {
				return config.StreamerConfig{
					ScreenId: trimmedScreenID,
					Folder:   strings.TrimSpace(streamerCfg.Folder),
					Password: strings.TrimSpace(streamerCfg.Password),
				}
			}
		}
	}

	return config.StreamerConfig{
		ScreenId: trimmedScreenID,
		Folder:   filepath.Join("Recordings", trimmedScreenID),
	}
}
