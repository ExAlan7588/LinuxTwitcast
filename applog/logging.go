package applog

import (
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

const defaultMaxLines = 2000
const defaultMaxAlertLines = 400

var (
	configMu        sync.Mutex
	alertFileMu     sync.Mutex
	capture         = newLogCapture(defaultMaxLines, defaultMaxAlertLines)
	currentLog      *os.File
	currentAlertLog *os.File
)

type ringBuffer struct {
	mu    sync.RWMutex
	lines []string
	max   int
}

func newRingBuffer(max int) *ringBuffer {
	return &ringBuffer{
		lines: make([]string, 0, max),
		max:   max,
	}
}

type logCapture struct {
	mu      sync.Mutex
	lines   *ringBuffer
	alerts  *ringBuffer
	partial string
}

func newLogCapture(maxLines, maxAlerts int) *logCapture {
	return &logCapture{
		lines:  newRingBuffer(maxLines),
		alerts: newRingBuffer(maxAlerts),
	}
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
	return snapshot
}

func (r *ringBuffer) FilteredLines(limit int, keep func(string) bool) ([]string, int) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	candidates := make([]string, 0, len(r.lines))
	candidates = append(candidates, r.lines...)

	filteredCount := 0
	kept := make([]string, 0, len(candidates))
	for _, line := range candidates {
		if keep != nil && !keep(line) {
			filteredCount++
			continue
		}
		kept = append(kept, line)
	}

	if limit <= 0 || limit > len(kept) {
		limit = len(kept)
	}

	start := len(kept) - limit
	snapshot := make([]string, limit)
	copy(snapshot, kept[start:])
	return snapshot, filteredCount
}

func (r *ringBuffer) push(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.lines) == r.max {
		copy(r.lines, r.lines[1:])
		r.lines[len(r.lines)-1] = line
		return
	}
	r.lines = append(r.lines, line)
}

func (c *logCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	chunk := strings.ReplaceAll(string(p), "\r\n", "\n")
	chunk = strings.ReplaceAll(chunk, "\r", "\n")
	text := c.partial + chunk
	parts := strings.Split(text, "\n")

	if strings.HasSuffix(text, "\n") {
		c.partial = ""
		parts = parts[:len(parts)-1]
	} else {
		c.partial = parts[len(parts)-1]
		parts = parts[:len(parts)-1]
	}

	for _, line := range parts {
		c.lines.push(line)
		if IsAlertLine(line) {
			c.alerts.push(line)
			writeAlertLogLine(line)
		}
	}

	return len(p), nil
}

func (c *logCapture) Lines(limit int) []string {
	lines := c.lines.Lines(limit)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.partial != "" {
		lines = append(lines, c.partial)
	}
	return lines
}

func (c *logCapture) FilteredLines(limit int, keep func(string) bool) ([]string, int) {
	lines := c.lines.Lines(0)
	c.mu.Lock()
	partial := c.partial
	c.mu.Unlock()
	if partial != "" {
		lines = append(lines, partial)
	}

	filteredCount := 0
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if keep != nil && !keep(line) {
			filteredCount++
			continue
		}
		kept = append(kept, line)
	}

	if limit <= 0 || limit > len(kept) {
		limit = len(kept)
	}

	start := len(kept) - limit
	snapshot := make([]string, limit)
	copy(snapshot, kept[start:])
	return snapshot, filteredCount
}

func (c *logCapture) AlertLines(limit int) []string {
	return c.alerts.Lines(limit)
}

func Configure(enableFile bool) error {
	configMu.Lock()
	defer configMu.Unlock()

	closeOpenFilesLocked()

	alertFile, err := os.OpenFile("error.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	currentAlertLog = alertFile

	writers := []io.Writer{os.Stdout, capture}
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

func Close() error {
	configMu.Lock()
	defer configMu.Unlock()
	return closeOpenFilesLocked()
}

func RecentLines(limit int) []string {
	return capture.Lines(limit)
}

func RecentLinesFiltered(limit int, keep func(string) bool) ([]string, int) {
	return capture.FilteredLines(limit, keep)
}

func RecentAlertLines(limit int) []string {
	return capture.AlertLines(limit)
}

func IsAlertLine(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "[error]") ||
		strings.HasPrefix(lower, "error ") ||
		strings.Contains(lower, " error ") ||
		strings.Contains(lower, "fatal") ||
		strings.Contains(lower, "panic") ||
		strings.Contains(lower, "failed")
}

func writeAlertLogLine(line string) {
	alertFileMu.Lock()
	defer alertFileMu.Unlock()
	if currentAlertLog == nil {
		return
	}
	_, _ = currentAlertLog.WriteString(line + "\n")
}

func closeOpenFilesLocked() error {
	var firstErr error
	if currentLog != nil {
		if err := currentLog.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		currentLog = nil
	}
	if currentAlertLog != nil {
		if err := currentAlertLog.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		currentAlertLog = nil
	}
	return firstErr
}
