package record

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/jzhang046/croned-twitcasting-recorder/state"
)

var (
	BackgroundProcessorWg sync.WaitGroup
)

// DiscordNotifier is the interface the record package calls to send notifications.
// The discord package's Notifier satisfies this interface.
type DiscordNotifier interface {
	NotifyStart(streamerName, streamTitle string)
	NotifyEnd(streamerName, streamTitle string)
}

type Sink interface {
	Chan() chan<- []byte
	Wait()
	Filename() string
}

type RecordConfig struct {
	Streamer         string
	Folder           string
	StreamUrlFetcher func(string) (string, string, string, error)
	SinkProvider     func(RecordContext) (Sink, error)
	StreamRecorder   func(RecordContext, chan<- []byte)
	RootContext      context.Context
	Notifier         DiscordNotifier // optional; nil disables Discord notifications
	PostProcessor    func(filename, streamerName, title string)
	OnSessionStart   func(SessionInfo)
	OnSessionEnd     func(SessionInfo)
}

type SessionInfo struct {
	Streamer     string    `json:"streamer"`
	StreamerName string    `json:"streamer_name"`
	Title        string    `json:"title"`
	Filename     string    `json:"filename"`
	StartedAt    time.Time `json:"started_at"`
	EndedAt      time.Time `json:"ended_at,omitempty"`
}

func ToRecordFunc(recordConfig *RecordConfig) func() {
	streamer := recordConfig.Streamer
	return func() {
		streamUrl, streamerName, title, err := recordConfig.StreamUrlFetcher(streamer)
		if err != nil {
			log.Printf("Error fetching stream URL for streamer [%s]: %v\n", streamer, err)
			return
		}
		log.Printf("Fetched stream URL for streamer [%s]: %s\n", streamer, streamUrl)
		recordCtx := newRecordContext(recordConfig.RootContext, streamer, streamUrl, streamerName, title, recordConfig.Folder)

		sink, err := recordConfig.SinkProvider(recordCtx)
		if err != nil {
			log.Println("Error creating recording file: ", err)
			return
		}

		session := SessionInfo{
			Streamer:     streamer,
			StreamerName: streamerName,
			Title:        title,
			Filename:     sink.Filename(),
			StartedAt:    time.Now(),
		}
		if recordConfig.OnSessionStart != nil {
			recordConfig.OnSessionStart(session)
		}

		// Update state to "recording"
		state.Update(streamer, "recording")

		// Notify Discord that recording has started
		if recordConfig.Notifier != nil {
			recordConfig.Notifier.NotifyStart(streamerName, title)
		}

		recordConfig.StreamRecorder(recordCtx, sink.Chan())

		// Wait for the sink file to finish writing before continuing
		sink.Wait()

		// Notify Discord that recording has ended
		if recordConfig.Notifier != nil {
			recordConfig.Notifier.NotifyEnd(streamerName, title)
		}
		if recordConfig.OnSessionEnd != nil {
			session.EndedAt = time.Now()
			recordConfig.OnSessionEnd(session)
		}

		// Execute post processor (e.g. Telegram upload & FFmpeg)
		if recordConfig.PostProcessor != nil {
			BackgroundProcessorWg.Add(1)
			go func() {
				defer BackgroundProcessorWg.Done()
				state.Update(streamer, "processing")
				recordConfig.PostProcessor(sink.Filename(), streamerName, title)
				// Clear state when post-processor completely finishes
				state.Clear(streamer)
			}()
		} else {
			// If no post-processor, clear state immediately
			state.Clear(streamer)
		}
	}
}
