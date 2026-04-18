package twitcasting

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrepareMovieDownloadReadsPlaylistFromMoviePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/mielu_ii/movie/833944593" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
			<html>
				<head>
					<title>會員回放 / Test - ミエル (@mielu_ii) - TwitCasting</title>
					<meta name="twitter:title" content="會員回放 / Test">
				</head>
				<body>
					<input type="hidden" id="created_unix_time" value="1776441152">
					<video
						data-movie-playlist='{"2":[{"startTime":0,"duration":1000,"source":{"url":"https:\/\/dl.example.test\/master.m3u8","type":"application\/x-mpegURL"}}]}'
						data-adaptive-bitrate-selected='2'>
					</video>
				</body>
			</html>
		`))
	}))
	defer server.Close()

	useTwitCastingTestHTTPClient(t, server)

	info, err := PrepareMovieDownload("mielu_ii", "833944593", "")
	if err != nil {
		t.Fatalf("PrepareMovieDownload() error = %v", err)
	}
	if info.Title != "會員回放 ／ Test" {
		t.Fatalf("Title = %q, want %q", info.Title, "會員回放 ／ Test")
	}
	if len(info.PlaylistURLs) != 1 || info.PlaylistURLs[0] != "https://dl.example.test/master.m3u8" {
		t.Fatalf("PlaylistURLs = %#v", info.PlaylistURLs)
	}
	if len(info.PlaylistDurations) != 1 || info.PlaylistDurations[0] != time.Second {
		t.Fatalf("PlaylistDurations = %#v", info.PlaylistDurations)
	}
	wantStartedAt := time.Unix(1776441152, 0)
	if !info.StartedAt.Equal(wantStartedAt) {
		t.Fatalf("StartedAt = %v, want %v", info.StartedAt, wantStartedAt)
	}
}

func TestPrepareMovieDownloadUnlocksPasswordProtectedMovie(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/mielu_ii/movie/833988018":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`
				<html>
					<head>
						<title>3日連続食べた🍄 / ♡ - ミエル (@mielu_ii) - TwitCasting</title>
						<meta name="twitter:title" content="3日連続食べた🍄 / ♡">
					</head>
					<body>
						<div class="tw-empty-state tw-player-page-lock-empty-state">
							<form method="POST">
								<input type="text" name="password" value="">
								<input type="hidden" name="cs_session_id" value="csrf-token-1">
							</form>
						</div>
					</body>
				</html>
			`))
		case r.Method == http.MethodPost && r.URL.Path == "/mielu_ii/movie/833988018":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm() error = %v", err)
			}
			if r.Form.Get("password") != "secret" {
				t.Fatalf("password = %q, want %q", r.Form.Get("password"), "secret")
			}
			if r.Form.Get("cs_session_id") != "csrf-token-1" {
				t.Fatalf("cs_session_id = %q, want %q", r.Form.Get("cs_session_id"), "csrf-token-1")
			}
			http.SetCookie(w, &http.Cookie{Name: "movie_pass", Value: "ok"})
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`
				<html>
					<head>
						<title>3日連続食べた🍄 / ♡ - ミエル (@mielu_ii) - TwitCasting</title>
						<meta name="twitter:title" content="3日連続食べた🍄 / ♡">
					</head>
					<body>
						<video
							data-movie-playlist='{"2":[{"startTime":0,"duration":1000,"source":{"url":"https:\/\/dl.example.test\/movie-master.m3u8","type":"application\/x-mpegURL"}}]}'
							data-adaptive-bitrate-selected='2'>
						</video>
					</body>
				</html>
			`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	useTwitCastingTestHTTPClient(t, server)

	info, err := PrepareMovieDownload("mielu_ii", "833988018", "secret")
	if err != nil {
		t.Fatalf("PrepareMovieDownload() error = %v", err)
	}
	if len(info.PlaylistURLs) != 1 || info.PlaylistURLs[0] != "https://dl.example.test/movie-master.m3u8" {
		t.Fatalf("PlaylistURLs = %#v", info.PlaylistURLs)
	}
	if !strings.Contains(info.CookieHeader, "movie_pass=ok") {
		t.Fatalf("CookieHeader = %q, want movie_pass cookie", info.CookieHeader)
	}
}

func TestBuildMovieDownloadArgsIncludesRefererAndCookie(t *testing.T) {
	args := buildMovieDownloadArgs(
		"https://twitcasting.tv/mielu_ii/movie/833944593",
		"tc_id=alice; movie_pass=ok",
		"https://dl.example.test/master.m3u8",
		"Recordings/mielu/movie.mp4",
	)

	joined := strings.Join(args, "\n")
	if !strings.Contains(joined, "Origin: https://twitcasting.tv") {
		t.Fatalf("args missing Origin header: %s", joined)
	}
	if !strings.Contains(joined, "Referer: https://twitcasting.tv/mielu_ii/movie/833944593") {
		t.Fatalf("args missing Referer header: %s", joined)
	}
	if !strings.Contains(joined, "Cookie: tc_id=alice; movie_pass=ok") {
		t.Fatalf("args missing Cookie header: %s", joined)
	}
	if !strings.Contains(joined, "-movflags") || !strings.Contains(joined, "+faststart") {
		t.Fatalf("args missing mp4 faststart flags: %s", joined)
	}
}

func TestPlannedMovieDownloadOutputsUsesMovieDateAndPartSuffix(t *testing.T) {
	info := MovieDownloadInfo{
		ScreenID:     "mielu_ii",
		StreamerName: "ミエル",
		Title:        "會員回放 ／ Test",
		StartedAt:    time.Date(2026, 4, 16, 23, 7, 32, 0, time.FixedZone("JST", 9*3600)),
		PlaylistURLs: []string{"https://dl.example.test/part1.m3u8", "https://dl.example.test/part2.m3u8"},
	}

	outputs := PlannedMovieDownloadOutputs(info, "Recordings/mielu_ii")
	want := []string{
		filepath.Join("Recordings", "mielu_ii", "[ミエル][2026-04-16]會員回放 ／ Test.part1.mp4"),
		filepath.Join("Recordings", "mielu_ii", "[ミエル][2026-04-16]會員回放 ／ Test.part2.mp4"),
	}
	if len(outputs) != len(want) {
		t.Fatalf("len(outputs) = %d, want %d", len(outputs), len(want))
	}
	for index := range want {
		if outputs[index] != want[index] {
			t.Fatalf("outputs[%d] = %q, want %q", index, outputs[index], want[index])
		}
	}
}

func TestDownloadMovieArchiveRequiresFFmpeg(t *testing.T) {
	originalLookPath := lookPathFFmpeg
	lookPathFFmpeg = func(string) (string, error) {
		return "", errors.New("not found")
	}
	t.Cleanup(func() {
		lookPathFFmpeg = originalLookPath
	})

	_, err := DownloadMovieArchive(MovieDownloadInfo{ScreenID: "mielu_ii", PlaylistURLs: []string{"https://dl.example.test/master.m3u8"}}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "ffmpeg is not available in PATH") {
		t.Fatalf("DownloadMovieArchive() error = %v, want ffmpeg missing error", err)
	}
}

func TestBuildMovieDownloadProgressUsesPlaylistDurations(t *testing.T) {
	info := MovieDownloadInfo{
		PlaylistURLs:      []string{"https://dl.example.test/part1.m3u8", "https://dl.example.test/part2.m3u8"},
		PlaylistDurations: []time.Duration{2 * time.Second, 2 * time.Second},
	}

	progress := buildMovieDownloadProgress(info, 1, "Recordings/mielu/movie.part2.mp4", 2*time.Second, time.Second, 4*time.Second)
	if progress.PartIndex != 1 || progress.PartCount != 2 {
		t.Fatalf("progress part = %d/%d", progress.PartIndex, progress.PartCount)
	}
	if progress.OutputPath != "Recordings/mielu/movie.part2.mp4" {
		t.Fatalf("progress output = %q", progress.OutputPath)
	}
	if progress.ProgressPercent != 75 {
		t.Fatalf("progress percent = %v, want 75", progress.ProgressPercent)
	}
}

func TestParseFFmpegProgressDuration(t *testing.T) {
	got, ok := parseFFmpegProgressDuration("out_time=00:01:02.500000")
	if !ok {
		t.Fatal("expected ffmpeg progress line to parse")
	}
	want := time.Minute + 2*time.Second + 500*time.Millisecond
	if got != want {
		t.Fatalf("duration = %v, want %v", got, want)
	}
}
