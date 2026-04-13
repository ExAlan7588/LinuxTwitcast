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
