package twitcasting

import "testing"

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

func TestParseStreamPageInfoExtractsAvatarURL(t *testing.T) {
	body := `
		<html>
			<head>
				<title>歡迎回來 - 測試主播 (@test_user) - TwitCasting</title>
				<meta content="//imagegw03.twitcasting.tv/avatar.jpg" property="og:image">
				<meta name="twitter:title" content="歡迎回來">
			</head>
		</html>
	`

	info := parseStreamPageInfo("test_user", body)
	if info.streamerName != "測試主播" {
		t.Fatalf("streamerName = %q, want %q", info.streamerName, "測試主播")
	}
	if info.title != "歡迎回來" {
		t.Fatalf("title = %q, want %q", info.title, "歡迎回來")
	}
	if info.avatarURL != "https://imagegw03.twitcasting.tv/avatar.jpg" {
		t.Fatalf("avatarURL = %q, want %q", info.avatarURL, "https://imagegw03.twitcasting.tv/avatar.jpg")
	}
}
