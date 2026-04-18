package twitcasting

import (
	"net/http"
	"os"
	"testing"
)

func TestNormalizeCookieInputAcceptsCookieHeader(t *testing.T) {
	got, err := NormalizeCookieInput("Cookie: tc_ss=session123; tc_id=alice")
	if err != nil {
		t.Fatalf("NormalizeCookieInput() error = %v", err)
	}
	if got != "tc_ss=session123; tc_id=alice" {
		t.Fatalf("NormalizeCookieInput() = %q, want %q", got, "tc_ss=session123; tc_id=alice")
	}
}

func TestNormalizeCookieInputAcceptsNetscapeCookieFile(t *testing.T) {
	input := "# Netscape HTTP Cookie File\n#HttpOnly_.twitcasting.tv\tTRUE\t/\tFALSE\t0\ttc_ss\tsession123\n.twitcasting.tv\tTRUE\t/\tFALSE\t0\ttc_id\talice\n.example.com\tTRUE\t/\tFALSE\t0\tignored\tvalue\n"
	got, err := NormalizeCookieInput(input)
	if err != nil {
		t.Fatalf("NormalizeCookieInput() error = %v", err)
	}
	if got != "tc_ss=session123; tc_id=alice" {
		t.Fatalf("NormalizeCookieInput() = %q, want %q", got, "tc_ss=session123; tc_id=alice")
	}
}

func TestApplyAuthToRequestSetsCookieForTwitCasting(t *testing.T) {
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

	if err := SaveAuthConfig(AuthConfig{CookieHeader: "tc_ss=session123; tc_id=alice"}); err != nil {
		t.Fatalf("SaveAuthConfig() error = %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, "https://twitcasting.tv/test_user", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	ApplyAuthToRequest(req)
	if got := req.Header.Get("Cookie"); got != "tc_ss=session123; tc_id=alice" {
		t.Fatalf("Cookie header = %q, want %q", got, "tc_ss=session123; tc_id=alice")
	}

	otherReq, err := http.NewRequest(http.MethodGet, "https://example.com/test", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	ApplyAuthToRequest(otherReq)
	if got := otherReq.Header.Get("Cookie"); got != "" {
		t.Fatalf("Cookie header for non-TwitCasting request = %q, want empty", got)
	}
}
