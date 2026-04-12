package telegram

import "testing"

func TestUploadMethodForPath(t *testing.T) {
	testCases := map[string]UploadMethod{
		"recording.m4a": UploadMethodAudio,
		"recording.mp3": UploadMethodAudio,
		"recording.ts":  UploadMethodDocument,
		"notes.txt":     UploadMethodDocument,
	}

	for path, want := range testCases {
		if got := uploadMethodForPath(path); got != want {
			t.Fatalf("uploadMethodForPath(%q) = %q, want %q", path, got, want)
		}
	}
}
