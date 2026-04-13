package record

import "context"

type RecordContext interface {
	// Done would be closed when work done.
	Done() <-chan struct{}

	// Err explains the reason when this context is Done().
	Err() error

	// Cancel cancels the record.
	Cancel()

	// GetStreamUrl returns the stream URL of this context.
	GetStreamUrl() string

	// GetStreamer returns streamer's screen ID of this context.
	GetStreamer() string

	// GetStreamerName returns the streamer display name cleanly.
	GetStreamerName() string

	// GetTitle returns the current stream title cleanly.
	GetTitle() string

	// GetFolder returns the target folder for recording.
	GetFolder() string

	// GetPassword returns the optional stream password for reconnect attempts.
	GetPassword() string
}

type recordContextImpl struct {
	ctx        context.Context
	cancelFunc context.CancelFunc
}

type contextKey string

const (
	streamerKey     = contextKey("streamer")
	streamUrlKey    = contextKey("streamUrl")
	streamerNameKey = contextKey("streamerName")
	titleKey        = contextKey("title")
	folderKey       = contextKey("folder")
	passwordKey     = contextKey("password")
)

func newRecordContext(ctx context.Context, streamer, streamUrl, streamerName, title, folder, password string) RecordContext {
	ctx, cancelFunc := context.WithCancel(ctx)
	ctx = context.WithValue(ctx, streamUrlKey, streamUrl)
	ctx = context.WithValue(ctx, streamerKey, streamer)
	ctx = context.WithValue(ctx, streamerNameKey, streamerName)
	ctx = context.WithValue(ctx, titleKey, title)
	ctx = context.WithValue(ctx, folderKey, folder)
	ctx = context.WithValue(ctx, passwordKey, password)
	return &recordContextImpl{ctx, cancelFunc}
}

func (ctxImpl *recordContextImpl) Done() <-chan struct{} {
	return ctxImpl.ctx.Done()
}

func (ctxImpl *recordContextImpl) Err() error {
	return ctxImpl.ctx.Err()
}

func (ctxImpl *recordContextImpl) Cancel() {
	ctxImpl.cancelFunc()
}

func (ctxImpl *recordContextImpl) GetStreamUrl() string {
	return ctxImpl.ctx.Value(streamUrlKey).(string)
}

func (ctxImpl *recordContextImpl) GetStreamer() string {
	return ctxImpl.ctx.Value(streamerKey).(string)
}

func (ctxImpl *recordContextImpl) GetStreamerName() string {
	if val := ctxImpl.ctx.Value(streamerNameKey); val != nil {
		return val.(string)
	}
	return ""
}

func (ctxImpl *recordContextImpl) GetTitle() string {
	if val := ctxImpl.ctx.Value(titleKey); val != nil {
		return val.(string)
	}
	return ""
}

func (ctxImpl *recordContextImpl) GetFolder() string {
	if val := ctxImpl.ctx.Value(folderKey); val != nil {
		return val.(string)
	}
	return ""
}

func (ctxImpl *recordContextImpl) GetPassword() string {
	if val := ctxImpl.ctx.Value(passwordKey); val != nil {
		return val.(string)
	}
	return ""
}
