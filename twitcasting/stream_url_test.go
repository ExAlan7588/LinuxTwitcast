package twitcasting

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jmoiron/jsonq"
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

func TestExtractMovieIDAcceptsNumericValue(t *testing.T) {
	payload := map[string]interface{}{}
	if err := json.Unmarshal([]byte(`{"movie":{"id":834555312}}`), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	movieID, err := extractMovieID(jsonq.NewQuery(payload))
	if err != nil {
		t.Fatalf("extractMovieID() error = %v", err)
	}
	if movieID != "834555312" {
		t.Fatalf("extractMovieID() = %q, want %q", movieID, "834555312")
	}
}

func TestParseStreamPageInfoPrefersLiveHeadingAndBroadcasterData(t *testing.T) {
	info := parseStreamPageInfo("iuuic1", `
		<html>
			<head>
				<title>ちの (@iUUic1) 的直播 - Twitcast</title>
				<meta name="twitter:title" content="注意喚起">
			</head>
			<body>
				<div data-broadcaster-name="ちの"></div>
				<div class="tw-player-page-title-title">
					<h2>現役JKが夜の雑だん</h2>
				</div>
			</body>
		</html>
	`)

	if info.streamerName != "ちの" {
		t.Fatalf("streamerName = %q, want %q", info.streamerName, "ちの")
	}
	if info.title != "現役JKが夜の雑だん" {
		t.Fatalf("title = %q, want %q", info.title, "現役JKが夜の雑だん")
	}
}

func TestParseStreamPageInfoPrefersMoreCompleteTwitterDescription(t *testing.T) {
	info := parseStreamPageInfo("iuuic1", `
		<html>
			<head>
				<title>ちの (@iUUic1) 's Live - Twitcast</title>
				<meta name="twitter:title" content="注意喚起">
				<meta name="twitter:description" content="注意喚起 (カワボ雑談)">
			</head>
			<body>
				<div data-broadcaster-name="ちの"></div>
				<div class="tw-player-page-title-title">
					<h2>注意喚起</h2>
				</div>
			</body>
		</html>
	`)

	if info.streamerName != "ちの" {
		t.Fatalf("streamerName = %q, want %q", info.streamerName, "ちの")
	}
	if info.title != "注意喚起 (カワボ雑談)" {
		t.Fatalf("title = %q, want %q", info.title, "注意喚起 (カワボ雑談)")
	}
}

func TestParseMoviePageInfoPrefersArchiveTitleMeta(t *testing.T) {
	info := parseMoviePageInfo("mielu_ii", `
		<html>
			<head>
				<title>♡ASMR - ミエル (@mielu_ii) - TwitCasting</title>
				<meta name="twitter:title" content="「超かぐや姫」同時視聴♩ / ♡ASMR">
				<meta property="og:title" content="old title">
			</head>
		</html>
	`)

	if info.streamerName != "ミエル" {
		t.Fatalf("streamerName = %q, want %q", info.streamerName, "ミエル")
	}
	want := "「超かぐや姫」同時視聴♩ ／ ♡ASMR"
	if info.title != want {
		t.Fatalf("title = %q, want %q", info.title, want)
	}
}

func TestParseMoviePageInfoPrefersMoreCompleteTwitterDescription(t *testing.T) {
	info := parseMoviePageInfo("iuuic1", `
		<html>
			<head>
				<title>注意喚起 - ちの (@iUUic1) - TwitCasting</title>
				<meta name="twitter:title" content="注意喚起">
				<meta name="twitter:description" content="注意喚起 (カワボ雑談)">
				<meta property="og:title" content="注意喚起">
			</head>
			<body>
				<div data-broadcaster-name="ちの"></div>
				<div class="tw-player-page-title-title">
					<h2>注意喚起</h2>
				</div>
			</body>
		</html>
	`)

	if info.title != "注意喚起 (カワボ雑談)" {
		t.Fatalf("title = %q, want %q", info.title, "注意喚起 (カワボ雑談)")
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

func TestSanitizeFilenameReplacesReservedCharacters(t *testing.T) {
	got := sanitizeFilename(`a/b\c:d*e?f"g<h>i|j`)
	want := `a／b＼c：d＊e？f”g＜h＞i｜j`
	if got != want {
		t.Fatalf("sanitizeFilename() = %q, want %q", got, want)
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

func TestGetWSStreamUrlWithPasswordPrefersMoviePageTitle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mielu_ii":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`
				<html>
					<head>
						<title>♡ - ミエル (@mielu_ii) - TwitCasting</title>
						<meta name="twitter:title" content="♡">
					</head>
				</html>
			`))
		case "/mielu_ii/movie/833988018":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`
				<html>
					<head>
						<title>3日連続食べた🍄 / ♡ - ミエル (@mielu_ii) - TwitCasting</title>
						<meta name="twitter:title" content="3日連続食べた🍄 / ♡">
					</head>
					<body>
						<div data-broadcaster-profile-image="//imagegw02.twitcasting.tv/mielu.jpg"></div>
					</body>
				</html>
			`))
		case "/streamserver.php":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"movie": {"live": true, "id": "833988018"},
				"llfmp4": {"streams": {"main": "wss://stream.example/live"}}
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	useTwitCastingTestHTTPClient(t, server)

	result, err := GetWSStreamUrlWithPassword("mielu_ii", "")
	if err != nil {
		t.Fatalf("GetWSStreamUrlWithPassword() error = %v", err)
	}
	if result.Title != "3日連続食べた🍄 ／ ♡" {
		t.Fatalf("Title = %q, want %q", result.Title, "3日連続食べた🍄 ／ ♡")
	}
	if result.StreamerName != "ミエル" {
		t.Fatalf("StreamerName = %q, want %q", result.StreamerName, "ミエル")
	}
	if result.StreamURL != "wss://stream.example/live" {
		t.Fatalf("StreamURL = %q, want %q", result.StreamURL, "wss://stream.example/live")
	}
}

func TestGetWSStreamUrlWithPasswordUsesMoviePageMemberOnlyMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/locked_user":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`
				<html>
					<head>
						<title>♡ - 鎖定主播 (@locked_user) - TwitCasting</title>
						<meta name="twitter:title" content="♡">
						<meta content="//imagegw03.twitcasting.tv/home-cover.jpg" property="og:image">
					</head>
				</html>
			`))
		case "/locked_user/movie/900001":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`
				<html>
					<head>
						<title>メン限本編 / つづき - 鎖定主播 (@locked_user) - TwitCasting</title>
						<meta name="twitter:title" content="メン限本編 / つづき">
						<meta content="//imagegw03.twitcasting.tv/member-cover.jpg" property="og:image">
					</head>
					<body>
						<div class="tw-player-page-lock-empty-state">
							<div>Members-only</div>
							<a href="/membershipjoinplans.php?u=locked_user">Join the membership</a>
						</div>
						<div data-broadcaster-profile-image="//imagegw02.twitcasting.tv/member-avatar.jpg"></div>
					</body>
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

	result, err := GetWSStreamUrlWithPassword("locked_user", "")
	if !errors.Is(err, ErrMemberOnlyLive) {
		t.Fatalf("GetWSStreamUrlWithPassword() error = %v, want %v", err, ErrMemberOnlyLive)
	}
	if result.Title != "メン限本編 ／ つづき" {
		t.Fatalf("Title = %q, want %q", result.Title, "メン限本編 ／ つづき")
	}
	if result.CoverURL != "https://imagegw03.twitcasting.tv/member-cover.jpg" {
		t.Fatalf("CoverURL = %q, want %q", result.CoverURL, "https://imagegw03.twitcasting.tv/member-cover.jpg")
	}
	if result.AvatarURL != "https://imagegw02.twitcasting.tv/member-avatar.jpg" {
		t.Fatalf("AvatarURL = %q, want %q", result.AvatarURL, "https://imagegw02.twitcasting.tv/member-avatar.jpg")
	}
}

func TestGetWSStreamUrlWithPasswordMarksAccessibleMembersOnlyLive(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
		InvalidateAuthCache()
	})

	if err := SaveAuthConfig(AuthConfig{CookieHeader: "tc_ss=session123"}); err != nil {
		t.Fatalf("SaveAuthConfig() error = %v", err)
	}

	authPage := `
		<html>
			<head>
				<title>ちの (@iUUic1) 的直播 - Twitcast</title>
				<meta name="twitter:title" content="注意喚起">
				<meta content="//imagegw03.twitcasting.tv/member-cover.jpg" property="og:image">
			</head>
			<body>
				<div data-broadcaster-name="ちの"></div>
				<div class="tw-player-page-title-title">
					<h2>現役JKが夜の雑だん</h2>
				</div>
				<div data-broadcaster-profile-image="//imagegw02.twitcasting.tv/member-avatar.jpg"></div>
			</body>
		</html>
	`
	publicLockedPage := `
		<html>
			<head>
				<title>ちの (@iUUic1) 的直播 - Twitcast</title>
			</head>
			<body>
				<div class="tw-player-page-lock-empty-state">
					<div>Members-only</div>
					<a href="/membershipjoinplans.php?u=access_user">Join the membership</a>
				</div>
			</body>
		</html>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasCookie := strings.Contains(r.Header.Get("Cookie"), "tc_ss=session123")

		switch r.URL.Path {
		case "/access_user":
			w.WriteHeader(http.StatusOK)
			if hasCookie {
				_, _ = w.Write([]byte(authPage))
				return
			}
			_, _ = w.Write([]byte(publicLockedPage))
		case "/access_user/movie/900002":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(authPage))
		case "/streamserver.php":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"movie": {"live": true, "id": "900002"},
				"llfmp4": {"streams": {"main": "wss://stream.example/live"}}
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	useTwitCastingTestHTTPClient(t, server)

	result, err := GetWSStreamUrlWithPassword("access_user", "")
	if err != nil {
		t.Fatalf("GetWSStreamUrlWithPassword() error = %v", err)
	}
	if !result.MemberOnly {
		t.Fatal("expected live to be marked members-only when only authenticated access can view it")
	}
	if result.StreamerName != "ちの" {
		t.Fatalf("StreamerName = %q, want %q", result.StreamerName, "ちの")
	}
	if result.Title != "現役JKが夜の雑だん" {
		t.Fatalf("Title = %q, want %q", result.Title, "現役JKが夜の雑だん")
	}
	if result.MovieID != "900002" {
		t.Fatalf("MovieID = %q, want %q", result.MovieID, "900002")
	}
}

func TestGetWSStreamUrlWithPasswordTreatsMembershipMovieFallbackAsMembersOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fallback_user":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`
				<html>
					<head>
						<title>ちの (@iUUic1) 的直播 - Twitcast</title>
						<meta name="twitter:title" content="注意喚起">
						<meta content="//imagegw03.twitcasting.tv/member-cover.jpg" property="og:image">
					</head>
					<body>
						<div class="tw-membership-button">
							<a href="/membershipjoindetail.php?u=fallback_user">Membership</a>
						</div>
						<div class="tw-player-page-title-title">
							<h2>現役JKが夜の雑だん</h2>
						</div>
						<div
							id="comment-list-app"
							data-broadcaster-name="ちの"
							data-movie-id="833497743"
							data-account-type="not_logged_in">
						</div>
					</body>
				</html>
			`))
		case "/fallback_user/movie/900003":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`
				<html>
					<head>
						<title>ちの (@iUUic1) 的直播 - Twitcast</title>
						<meta name="twitter:title" content="注意喚起">
					</head>
					<body>
						<div class="tw-membership-button">
							<a href="/membershipjoindetail.php?u=fallback_user">Membership</a>
						</div>
						<div
							id="comment-list-app"
							data-broadcaster-name="ちの"
							data-movie-id="833497743"
							data-account-type="not_logged_in">
						</div>
					</body>
				</html>
			`))
		case "/streamserver.php":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"movie": {"live": true, "id": "900003"},
				"llfmp4": {"streams": {"main": "wss://stream.example/live"}}
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	useTwitCastingTestHTTPClient(t, server)

	result, err := GetWSStreamUrlWithPassword("fallback_user", "")
	if !errors.Is(err, ErrMemberOnlyLive) {
		t.Fatalf("GetWSStreamUrlWithPassword() error = %v, want %v", err, ErrMemberOnlyLive)
	}
	if !result.MemberOnly {
		t.Fatal("expected fallback live to be marked members-only")
	}
	if result.MovieID != "900003" {
		t.Fatalf("MovieID = %q, want %q", result.MovieID, "900003")
	}
	if result.Title != "" {
		t.Fatalf("Title = %q, want empty title for inaccessible members-only fallback", result.Title)
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
