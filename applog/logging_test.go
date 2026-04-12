package applog

import "testing"

func TestFilteredLinesKeepsRecentUsefulLines(t *testing.T) {
	ring := newRingBuffer(10)
	for _, line := range []string{
		"first useful",
		"noise offline 1",
		"noise offline 2",
		"second useful",
		"noise offline 3",
		"third useful",
	} {
		ring.push(line)
	}

	lines, filteredCount := ring.FilteredLines(2, func(line string) bool {
		return line != "noise offline 1" && line != "noise offline 2" && line != "noise offline 3"
	})

	if filteredCount != 3 {
		t.Fatalf("expected 3 filtered lines, got %d", filteredCount)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 kept lines, got %d", len(lines))
	}
	if lines[0] != "second useful" || lines[1] != "third useful" {
		t.Fatalf("unexpected lines: %#v", lines)
	}
}
