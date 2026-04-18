package twitcasting

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseManualRecordTargetSupportsMovieURL(t *testing.T) {
	target, err := ParseManualRecordTarget("https://twitcasting.tv/mielu_ii/movie/833988018")
	if err != nil {
		t.Fatalf("ParseManualRecordTarget() error = %v", err)
	}
	if target.ScreenID != "mielu_ii" {
		t.Fatalf("ScreenID = %q, want %q", target.ScreenID, "mielu_ii")
	}
	if target.MovieID != "833988018" {
		t.Fatalf("MovieID = %q, want %q", target.MovieID, "833988018")
	}
}

func TestLookupManualRecordingTargetRejectsArchivedMovieURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mielu_ii":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`
				<html>
					<head><title>現行直播 - ミエル (@mielu_ii) - TwitCasting</title></head>
				</html>
			`))
		case "/mielu_ii/movie/900001":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`
				<html>
					<head><title>現行直播 - ミエル (@mielu_ii) - TwitCasting</title></head>
				</html>
			`))
		case "/streamserver.php":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"movie": {"live": true, "id": "900001"},
				"llfmp4": {"streams": {"main": "wss://stream.example/live"}}
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	useTwitCastingTestHTTPClient(t, server)

	_, _, err := LookupManualRecordingTarget("https://twitcasting.tv/mielu_ii/movie/833988018", "")
	if !errors.Is(err, ErrMovieDownloadUnsupported) {
		t.Fatalf("LookupManualRecordingTarget() error = %v, want %v", err, ErrMovieDownloadUnsupported)
	}
}
