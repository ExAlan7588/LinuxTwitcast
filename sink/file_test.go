package sink

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jzhang046/croned-twitcasting-recorder/record"
)

type testRecordContext struct {
	streamer     string
	streamerName string
	title        string
	folder       string
}

func (c testRecordContext) Done() <-chan struct{}   { return make(chan struct{}) }
func (c testRecordContext) Err() error              { return nil }
func (c testRecordContext) Cancel()                 {}
func (c testRecordContext) GetStreamUrl() string    { return "" }
func (c testRecordContext) GetStreamer() string     { return c.streamer }
func (c testRecordContext) GetStreamerName() string { return c.streamerName }
func (c testRecordContext) GetTitle() string        { return c.title }
func (c testRecordContext) GetFolder() string       { return c.folder }
func (c testRecordContext) GetPassword() string     { return "" }

func TestOpenRecordingFileUsesNumberedSuffixOnCollision(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "streamer-20260506-1200.ts")
	if err := os.WriteFile(base, []byte("existing"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	candidate, f, err := openRecordingFile(base)
	if err != nil {
		t.Fatalf("openRecordingFile() error = %v", err)
	}
	t.Cleanup(func() {
		_ = f.Close()
	})

	want := filepath.Join(dir, "streamer-20260506-1200 (2).ts")
	if candidate != want {
		t.Fatalf("candidate = %q, want %q", candidate, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
}

func TestNewFileSinkKeepsBaseNameAndAvoidsAppendCollision(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "streamer-20260506-1200.ts")
	if err := os.WriteFile(base, []byte("existing"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	sink, err := NewFileSink(testRecordContext{
		streamer:     "streamer",
		streamerName: "Streamer",
		title:        "",
		folder:       dir,
	})
	if err != nil {
		t.Fatalf("NewFileSink() error = %v", err)
	}
	fileSink, ok := sink.(*FileSink)
	if !ok {
		t.Fatalf("NewFileSink() returned %T, want *FileSink", sink)
	}
	close(fileSink.sinkChan)
	fileSink.Wait()

	got := sink.Filename()
	if got == base {
		t.Fatal("expected collision-safe filename, got existing base path")
	}
}

var _ record.RecordContext = testRecordContext{}
