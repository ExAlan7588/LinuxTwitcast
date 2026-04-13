package twitcasting

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubRecordContext struct {
	ctx      context.Context
	cancel   context.CancelFunc
	streamer string
	password string
}

func newStubRecordContext(streamer string) *stubRecordContext {
	ctx, cancel := context.WithCancel(context.Background())
	return &stubRecordContext{
		ctx:      ctx,
		cancel:   cancel,
		streamer: streamer,
	}
}

func (s *stubRecordContext) Done() <-chan struct{} { return s.ctx.Done() }
func (s *stubRecordContext) Err() error            { return s.ctx.Err() }
func (s *stubRecordContext) Cancel()               { s.cancel() }
func (s *stubRecordContext) GetStreamUrl() string  { return "" }
func (s *stubRecordContext) GetStreamer() string   { return s.streamer }
func (s *stubRecordContext) GetStreamerName() string {
	return ""
}
func (s *stubRecordContext) GetTitle() string  { return "" }
func (s *stubRecordContext) GetFolder() string { return "" }
func (s *stubRecordContext) GetPassword() string {
	return s.password
}

func TestWaitForReconnectURLRetriesWithinGracePeriod(t *testing.T) {
	originalGrace := wsReconnectGracePeriod
	originalRetry := wsReconnectRetryInterval
	originalFetcher := wsReconnectURLFetcher
	t.Cleanup(func() {
		wsReconnectGracePeriod = originalGrace
		wsReconnectRetryInterval = originalRetry
		wsReconnectURLFetcher = originalFetcher
	})

	wsReconnectGracePeriod = 80 * time.Millisecond
	wsReconnectRetryInterval = 10 * time.Millisecond

	attempts := 0
	wsReconnectURLFetcher = func(streamer, password string) (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("temporary disconnect")
		}
		return "wss://example.test/reconnected", nil
	}

	recordCtx := newStubRecordContext("streamer-a")
	got, err := waitForReconnectURL(recordCtx, "streamer-a", errors.New("initial disconnect"))
	if err != nil {
		t.Fatalf("expected reconnect success, got error: %v", err)
	}
	if got != "wss://example.test/reconnected" {
		t.Fatalf("unexpected reconnect URL: %s", got)
	}
	if attempts < 3 {
		t.Fatalf("expected retries before success, attempts=%d", attempts)
	}
}

func TestWaitForReconnectURLTimesOutAfterGracePeriod(t *testing.T) {
	originalGrace := wsReconnectGracePeriod
	originalRetry := wsReconnectRetryInterval
	originalFetcher := wsReconnectURLFetcher
	t.Cleanup(func() {
		wsReconnectGracePeriod = originalGrace
		wsReconnectRetryInterval = originalRetry
		wsReconnectURLFetcher = originalFetcher
	})

	wsReconnectGracePeriod = 35 * time.Millisecond
	wsReconnectRetryInterval = 10 * time.Millisecond
	wsReconnectURLFetcher = func(streamer, password string) (string, error) {
		return "", errors.New("still offline")
	}

	recordCtx := newStubRecordContext("streamer-b")
	if _, err := waitForReconnectURL(recordCtx, "streamer-b", errors.New("initial disconnect")); err == nil {
		t.Fatal("expected reconnect timeout error")
	}
}
