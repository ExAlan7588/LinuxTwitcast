package sink

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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
	streamerName := recordCtx.GetStreamerName()
	if streamerName == "" {
		streamerName = recordCtx.GetStreamer()
	}

	title := recordCtx.GetTitle()

	var filename string
	if title != "" {
		filename = fmt.Sprintf("[%s][%s]%s.ts", streamerName, time.Now().Format("2006-01-02"), title)
	} else {
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

	// If the file doesn't exist, create it, or append to the file
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0664)
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
