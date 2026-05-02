package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFormatRunSummary(t *testing.T) {
	got := formatRunSummary(runTimings{
		sync:    1200 * time.Millisecond,
		command: 3400 * time.Millisecond,
		syncSteps: syncStepTimings{
			manifest: 20 * time.Millisecond,
			rsync:    900 * time.Millisecond,
		},
		syncSkipped: true,
	}, 5*time.Second, 7)
	for _, want := range []string{
		"run summary",
		"sync=1.2s",
		"command=3.4s",
		"total=5s",
		"sync_skipped=true",
		"exit=7",
		"sync_steps=manifest:20ms,rsync:900ms",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary missing %q in %q", want, got)
		}
	}
}

func TestFormatRunSummaryIncludesGitHydrateSkipReason(t *testing.T) {
	got := formatRunSummary(runTimings{
		sync: 2 * time.Second,
		syncSteps: syncStepTimings{
			gitHydrateSkipped:    true,
			gitHydrateSkipReason: "remote base current",
		},
	}, 3*time.Second, 0)
	if !strings.Contains(got, "git_hydrate:skipped_remote_base_current") {
		t.Fatalf("summary missing git hydrate skip reason: %q", got)
	}
}

func TestTimingJSONShape(t *testing.T) {
	var buf bytes.Buffer
	err := writeTimingJSON(&buf, timingReportFromRun("aws", "cbx_123", "blue-crab", runTimings{
		sync:    1200 * time.Millisecond,
		command: 3400 * time.Millisecond,
		syncSteps: syncStepTimings{
			rsync:                900 * time.Millisecond,
			gitHydrateSkipped:    true,
			gitHydrateSkipReason: "marker base current",
		},
		syncSkipped: true,
	}, 5*time.Second, 7))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Provider    string `json:"provider"`
		LeaseID     string `json:"leaseId"`
		SyncMs      int64  `json:"syncMs"`
		CommandMs   int64  `json:"commandMs"`
		TotalMs     int64  `json:"totalMs"`
		ExitCode    int    `json:"exitCode"`
		SyncSkipped bool   `json:"syncSkipped"`
		SyncPhases  []struct {
			Name    string `json:"name"`
			Ms      int64  `json:"ms"`
			Skipped bool   `json:"skipped"`
			Reason  string `json:"reason"`
		} `json:"syncPhases"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Provider != "aws" || got.LeaseID != "cbx_123" || got.SyncMs != 1200 || got.CommandMs != 3400 || got.TotalMs != 5000 || got.ExitCode != 7 || !got.SyncSkipped {
		t.Fatalf("unexpected report: %#v", got)
	}
	if len(got.SyncPhases) != 2 || got.SyncPhases[1].Name != "git_hydrate" || !got.SyncPhases[1].Skipped || got.SyncPhases[1].Reason != "marker base current" {
		t.Fatalf("unexpected phases: %#v", got.SyncPhases)
	}
}

func TestCommandNeedsHydrationHint(t *testing.T) {
	if !commandNeedsHydrationHint([]string{"env NODE_OPTIONS=--max-old-space-size=4096 pnpm test"}, true) {
		t.Fatal("expected shell pnpm command to need hydration hint")
	}
	if commandNeedsHydrationHint([]string{"go", "test", "./..."}, false) {
		t.Fatal("go test should not need hydration hint")
	}
}
