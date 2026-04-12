package applog

import (
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

const defaultMaxLines = 400

var (
	configMu   sync.Mutex
	buffer     = newRingBuffer(defaultMaxLines)
	currentLog *os.File
)

type ringBuffer struct {
	mu      sync.RWMutex
	lines   []string
	max     int
	partial string
}

func newRingBuffer(max int) *ringBuffer {
	return &ringBuffer{
		lines: make([]string, 0, max),
		max:   max,
	}
}

func (r *ringBuffer) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	chunk := strings.ReplaceAll(string(p), "\r\n", "\n")
	chunk = strings.ReplaceAll(chunk, "\r", "\n")
	text := r.partial + chunk
	parts := strings.Split(text, "\n")

	if strings.HasSuffix(text, "\n") {
		r.partial = ""
		parts = parts[:len(parts)-1]
	} else {
		r.partial = parts[len(parts)-1]
		parts = parts[:len(parts)-1]
	}

	for _, line := range parts {
		r.push(line)
	}

	return len(p), nil
}

func (r *ringBuffer) Lines(limit int) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 || limit > len(r.lines) {
		limit = len(r.lines)
	}

	start := len(r.lines) - limit
	snapshot := make([]string, limit)
	copy(snapshot, r.lines[start:])

	if r.partial != "" {
		snapshot = append(snapshot, r.partial)
	}

	return snapshot
}

func (r *ringBuffer) push(line string) {
	if len(r.lines) == r.max {
		copy(r.lines, r.lines[1:])
		r.lines[len(r.lines)-1] = line
		return
	}
	r.lines = append(r.lines, line)
}

func Configure(enableFile bool) error {
	configMu.Lock()
	defer configMu.Unlock()

	if currentLog != nil {
		_ = currentLog.Close()
		currentLog = nil
	}

	writers := []io.Writer{os.Stdout, buffer}
	if enableFile {
		file, err := os.OpenFile("app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			return err
		}
		currentLog = file
		writers = append(writers, file)
	}

	log.SetOutput(io.MultiWriter(writers...))
	return nil
}

func ConfigureFromEnv() error {
	return Configure(os.Getenv("TWITCAST_LOG_CONSOLE") == "1")
}

func RecentLines(limit int) []string {
	return buffer.Lines(limit)
}
