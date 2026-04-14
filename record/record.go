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
	NotifyStart(session SessionInfo)
	NotifyEnd(session SessionInfo)
}

type Sink interface {
	Chan() chan<- []byte
	Wait()
	Filename() string
}

type RecordConfig struct {
	Streamer         string
	Folder           string
	Password         string
	StreamUrlFetcher func(string) (StreamLookupResult, error)
	SinkProvider     func(RecordContext) (Sink, error)
	StreamRecorder   func(RecordContext, chan<- []byte)
	RootContext      context.Context
	Notifier         DiscordNotifier // optional; nil disables Discord notifications
	PostProcessor    func(SessionInfo)
	OnSessionStart   func(SessionInfo)
	OnSessionEnd     func(SessionInfo)
	OnStreamLookup   func(streamer string, err error)
}

type StreamLookupResult struct {
	StreamURL    string
	StreamerName string
	Title        string
	AvatarURL    string
}

type SessionInfo struct {
	Streamer     string    `json:"streamer"`
	StreamerName string    `json:"streamer_name"`
	Title        string    `json:"title"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
	Filename     string    `json:"filename"`
	StartedAt    time.Time `json:"started_at"`
	EndedAt      time.Time `json:"ended_at,omitempty"`
}

func ToRecordFunc(recordConfig *RecordConfig) func() {
	streamer := recordConfig.Streamer
	return func() {
		lookup, err := recordConfig.StreamUrlFetcher(streamer)
		if recordConfig.OnStreamLookup != nil {
			recordConfig.OnStreamLookup(streamer, err)
		}
		if err != nil {
			log.Printf("Error fetching stream URL for streamer [%s]: %v\n", streamer, err)
			return
		}
		log.Printf("Fetched stream URL for streamer [%s]: %s\n", streamer, lookup.StreamURL)
		recordCtx := newRecordContext(recordConfig.RootContext, streamer, lookup.StreamURL, lookup.StreamerName, lookup.Title, recordConfig.Folder, recordConfig.Password)

		sink, err := recordConfig.SinkProvider(recordCtx)
		if err != nil {
			log.Println("Error creating recording file: ", err)
			return
		}

		session := SessionInfo{
			Streamer:     streamer,
			StreamerName: lookup.StreamerName,
			Title:        lookup.Title,
			AvatarURL:    lookup.AvatarURL,
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
			recordConfig.Notifier.NotifyStart(session)
		}

		recordConfig.StreamRecorder(recordCtx, sink.Chan())

		// Wait for the sink file to finish writing before continuing
		sink.Wait()

		// Notify Discord that recording has ended
		if recordConfig.Notifier != nil {
			recordConfig.Notifier.NotifyEnd(session)
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
				recordConfig.PostProcessor(session)
				// Clear state when post-processor completely finishes
				state.Clear(streamer)
			}()
		} else {
			// If no post-processor, clear state immediately
			state.Clear(streamer)
		}
	}
}
