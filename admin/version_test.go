package admin

import "testing"

func TestNormalizeRepositoryURL(t *testing.T) {
	testCases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "https remote",
			raw:  "https://github.com/ExAlan7588/LinuxTwitcast.git",
			want: "https://github.com/ExAlan7588/LinuxTwitcast",
		},
		{
			name: "ssh scp remote",
			raw:  "git@github.com:ExAlan7588/LinuxTwitcast.git",
			want: "https://github.com/ExAlan7588/LinuxTwitcast",
		},
		{
			name: "ssh url remote",
			raw:  "ssh://git@github.com/ExAlan7588/LinuxTwitcast.git",
			want: "https://github.com/ExAlan7588/LinuxTwitcast",
		},
		{
			name: "blank",
			raw:  "",
			want: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeRepositoryURL(tc.raw)
			if got != tc.want {
				t.Fatalf("normalizeRepositoryURL(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestParseRemoteRef(t *testing.T) {
	got := parseRemoteRef("4317d16fcb6aef1f8f2d96c6f13acabe9f3f6f0d\trefs/heads/main")
	want := "4317d16fcb6aef1f8f2d96c6f13acabe9f3f6f0d"
	if got != want {
		t.Fatalf("parseRemoteRef() = %q, want %q", got, want)
	}
}

func TestCheckForUpdatesWithoutGitMetadata(t *testing.T) {
	result := CheckForUpdates(t.TempDir(), BuildInfo{Version: defaultVersion})
	if result.UpdateAvailable {
		t.Fatal("expected update_available to be false")
	}
	if result.Message == "" {
		t.Fatal("expected a human-readable message")
	}
}
