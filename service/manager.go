package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
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
	LastError        string               `json:"last_error,omitempty"`
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
	lastError        string
}

func NewManager() *Manager {
	return &Manager{
		active: make(map[string]record.SessionInfo),
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
	if err := config.Validate(cfg); err != nil {
		m.storeError(err)
		return err
	}
	if config.EnabledStreamers(cfg) == 0 {
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

		var streamerNotifier record.DiscordNotifier
		if notifier := discord.NewNotifierFromConfig(discordCfg, streamerCfg.ScreenId); notifier != nil {
			streamerNotifier = notifier
		}

		_, err := scheduler.AddFunc(
			streamerCfg.Schedule,
			record.ToRecordFunc(&record.RecordConfig{
				Streamer:         streamerCfg.ScreenId,
				Folder:           streamerCfg.Folder,
				StreamUrlFetcher: twitcasting.GetWSStreamUrl,
				SinkProvider:     sink.NewFileSink,
				StreamRecorder:   twitcasting.RecordWS,
				RootContext:      rootCtx,
				Notifier:         streamerNotifier,
				PostProcessor: func(filename, streamerName, title string) {
					telegram.Process(telegramCfg, filename, streamerName, title)
				},
				OnSessionStart: m.handleSessionStart,
				OnSessionEnd:   m.handleSessionEnd,
			}),
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
	m.enabledStreamers = config.EnabledStreamers(cfg)
	m.scheduledJobs = scheduledJobs
	m.active = make(map[string]record.SessionInfo)
	m.lastError = ""
	m.mu.Unlock()

	scheduler.Start()
	log.Println("Croned recorder started")
	return nil
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

	status := Status{
		Running:          m.running,
		Stopping:         m.stopping,
		StartedAt:        m.startedAt,
		TotalStreamers:   m.totalStreamers,
		EnabledStreamers: m.enabledStreamers,
		ScheduledJobs:    m.scheduledJobs,
		ActiveRecordings: active,
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
}

func (m *Manager) handleSessionEnd(session record.SessionInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.active, session.Streamer)
}

func (m *Manager) storeError(err error) {
	if err == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastError = err.Error()
}
