package admin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildFileRootsOnlyReturnsRecordings(t *testing.T) {
	rootDir := t.TempDir()
	recordingsRoot := filepath.Join(rootDir, "Recordings")
	if err := os.MkdirAll(recordingsRoot, 0755); err != nil {
		t.Fatalf("mkdir recordings root: %v", err)
	}

	roots := BuildFileRoots(rootDir)
	if len(roots) != 1 {
		t.Fatalf("len(BuildFileRoots()) = %d, want 1", len(roots))
	}
	if roots[0].Label != "Recordings" {
		t.Fatalf("root label = %q, want %q", roots[0].Label, "Recordings")
	}
	if roots[0].Root != recordingsRoot {
		t.Fatalf("root path = %q, want %q", roots[0].Root, recordingsRoot)
	}
	if !roots[0].Exists {
		t.Fatal("expected recordings root to be marked as existing")
	}
}
