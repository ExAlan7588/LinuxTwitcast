package sink

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jzhang046/croned-twitcasting-recorder/record"
)

const (
	timeFormat     = "20060102-1504"
	sinkChanBuffer = 16
)

type FileSink struct {
	sinkChan chan []byte
	filename string
	done     chan struct{}
}

func (s *FileSink) Chan() chan<- []byte {
	return s.sinkChan
}

func (s *FileSink) Filename() string {
	return s.filename
}

func (s *FileSink) Wait() {
	<-s.done
}

func NewFileSink(recordCtx record.RecordContext) (record.Sink, error) {
	session := record.SessionInfo{
		Streamer:     recordCtx.GetStreamer(),
		StreamerName: recordCtx.GetStreamerName(),
		Title:        recordCtx.GetTitle(),
		StartedAt:    time.Now(),
	}

	var filename string
	if strings.TrimSpace(session.Title) != "" {
		filename = fmt.Sprintf("%s.ts", record.FormattedMediaName(session))
	} else {
		streamerName := session.StreamerName
		if streamerName == "" {
			streamerName = session.Streamer
		}
		filename = fmt.Sprintf("%s-%s.ts", streamerName, time.Now().Format(timeFormat))
	}

	folder := recordCtx.GetFolder()
	if folder != "" {
		if err := os.MkdirAll(folder, os.ModePerm); err != nil {
			log.Printf("Failed to create folder [%s]: %v\n", folder, err)
		} else {
			filename = filepath.Join(folder, filename)
		}
	}

	filename, f, err := openRecordingFile(filename)
	if err != nil {
		return nil, err
	}
	log.Printf("Recording file %s", filename)

	sink := &FileSink{
		sinkChan: make(chan []byte, sinkChanBuffer),
		filename: filename,
		done:     make(chan struct{}),
	}

	go func() {
		defer f.Close()
		defer close(sink.done)
		for data := range sink.sinkChan {
			if _, err = f.Write(data); err != nil {
				log.Printf("Error writing recording file %s: %v\n", filename, err)
				recordCtx.Cancel()
				return
			}
		}
		log.Printf("Completed writing all data to %s\n", filename)
	}()

	return sink, nil
}

func openRecordingFile(filename string) (string, *os.File, error) {
	for index := 1; ; index++ {
		candidate := recordingFilenameWithSuffix(filename, index)
		f, err := os.OpenFile(candidate, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0664)
		if err == nil {
			return candidate, f, nil
		}
		if !os.IsExist(err) {
			return "", nil, err
		}
	}
}

func recordingFilenameWithSuffix(filename string, index int) string {
	if index <= 1 {
		return filename
	}

	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	return fmt.Sprintf("%s (%d)%s", base, index, ext)
}
