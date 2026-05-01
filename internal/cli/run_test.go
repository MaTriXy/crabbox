package cli

import (
	"strings"
	"testing"
	"time"
)

func TestFormatRunSummary(t *testing.T) {
	got := formatRunSummary(runTimings{
		sync:        1200 * time.Millisecond,
		command:     3400 * time.Millisecond,
		syncSkipped: true,
	}, 5*time.Second, 7)
	for _, want := range []string{
		"run summary",
		"sync=1.2s",
		"command=3.4s",
		"total=5s",
		"sync_skipped=true",
		"exit=7",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary missing %q in %q", want, got)
		}
	}
}
