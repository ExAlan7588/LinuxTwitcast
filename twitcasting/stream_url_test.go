package twitcasting

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRequiresStreamPassword(t *testing.T) {
	body := `
		<div class="tw-empty-state tw-player-page-lock-empty-state">
			<form method="POST">
				<input type="text" name="password" value="">
			</form>
		</div>
	`

	if !requiresStreamPassword(body) {
		t.Fatal("expected password-protected page to be detected")
	}

	if requiresStreamPassword(`<form method="POST"><input type="text" name="nickname"></form>`) {
		t.Fatal("unexpected password detection on a normal form")
	}
}

func TestAppendPasswordToken(t *testing.T) {
	got := appendPasswordToken("wss://example.test/stream?id=1", "secret")
	want := "wss://example.test/stream?id=1&word=5ebe2294ecd0e0f08eab7690d2a6ee69"
	if got != want {
		t.Fatalf("appendPasswordToken() = %q, want %q", got, want)
	}

	if unchanged := appendPasswordToken("wss://example.test/stream?id=1", ""); unchanged != "wss://example.test/stream?id=1" {
		t.Fatalf("expected URL without password to remain unchanged, got %q", unchanged)
	}
}

func TestParseStreamPageInfoSeparatesAvatarAndCover(t *testing.T) {
	body := `
		<html>
			<head>
				<title>歡迎回來 - 測試主播 (@test_user) - TwitCasting</title>
				<meta content="//imagegw03.twitcasting.tv/stream-cover.jpg" property="og:image">
				<meta name="twitter:title" content="歡迎回來">
			</head>
			<body>
				<div
					data-broadcaster-profile-image="//imagegw02.twitcasting.tv/profile-avatar.jpg"
					data-broadcaster-id="test_user">
				</div>
			</body>
		</html>
	`

	info := parseStreamPageInfo("test_user", body)
	if info.streamerName != "測試主播" {
		t.Fatalf("streamerName = %q, want %q", info.streamerName, "測試主播")
	}
	if info.title != "歡迎回來" {
		t.Fatalf("title = %q, want %q", info.title, "歡迎回來")
	}
	if info.avatarURL != "https://imagegw02.twitcasting.tv/profile-avatar.jpg" {
		t.Fatalf("avatarURL = %q, want %q", info.avatarURL, "https://imagegw02.twitcasting.tv/profile-avatar.jpg")
	}
	if info.coverURL != "https://imagegw03.twitcasting.tv/stream-cover.jpg" {
		t.Fatalf("coverURL = %q, want %q", info.coverURL, "https://imagegw03.twitcasting.tv/stream-cover.jpg")
	}
}

func TestParseStreamPageInfoKeepsCoverWhenAvatarMissing(t *testing.T) {
	body := `
		<html>
			<head>
				<title>晚安台 - 測試主播 (@test_user) - TwitCasting</title>
				<meta content="//imagegw03.twitcasting.tv/stream-cover.jpg" property="og:image">
				<meta name="twitter:title" content="晚安台">
			</head>
		</html>
	`

	info := parseStreamPageInfo("test_user", body)
	if info.avatarURL != "" {
		t.Fatalf("avatarURL = %q, want empty avatar when only cover meta exists", info.avatarURL)
	}
	if info.coverURL != "https://imagegw03.twitcasting.tv/stream-cover.jpg" {
		t.Fatalf("coverURL = %q, want %q", info.coverURL, "https://imagegw03.twitcasting.tv/stream-cover.jpg")
	}
}

func TestLookupStreamerProfileParsesValidPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/alice" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
			<html>
				<head>
					<title>歌回 - Alice/主播 (@alice) - TwitCasting</title>
					<meta name="twitter:title" content="歌回">
				</head>
				<body>
					<div data-broadcaster-profile-image="//imagegw02.twitcasting.tv/alice.jpg"></div>
				</body>
			</html>
		`))
	}))
	defer server.Close()

	useTwitCastingTestHTTPClient(t, server)

	profile, err := LookupStreamerProfile("alice")
	if err != nil {
		t.Fatalf("LookupStreamerProfile() error = %v", err)
	}
	if profile.ScreenID != "alice" {
		t.Fatalf("ScreenID = %q, want %q", profile.ScreenID, "alice")
	}
	if profile.StreamerName != "Alice／主播" {
		t.Fatalf("StreamerName = %q, want %q", profile.StreamerName, "Alice／主播")
	}
	if profile.Title != "歌回" {
		t.Fatalf("Title = %q, want %q", profile.Title, "歌回")
	}
	if profile.AvatarURL != "https://imagegw02.twitcasting.tv/alice.jpg" {
		t.Fatalf("AvatarURL = %q, want %q", profile.AvatarURL, "https://imagegw02.twitcasting.tv/alice.jpg")
	}
}

func TestLookupStreamerProfileReturnsNotFoundOn404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	useTwitCastingTestHTTPClient(t, server)

	_, err := LookupStreamerProfile("missing-user")
	if !errors.Is(err, ErrStreamerNotFound) {
		t.Fatalf("LookupStreamerProfile() error = %v, want %v", err, ErrStreamerNotFound)
	}
}

func useTwitCastingTestHTTPClient(t *testing.T, server *httptest.Server) {
	t.Helper()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	originalClient := httpClient
	client := server.Client()
	baseTransport := client.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	client.Transport = rewriteTwitCastingTransport{
		target: targetURL,
		base:   baseTransport,
	}
	client.Timeout = requestTimeout
	httpClient = client

	t.Cleanup(func() {
		httpClient = originalClient
	})
}

type rewriteTwitCastingTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (t rewriteTwitCastingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	return t.base.RoundTrip(clone)
}
