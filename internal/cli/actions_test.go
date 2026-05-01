package cli

import (
	"strings"
	"testing"
)

func TestParseGitHubRepo(t *testing.T) {
	tests := map[string]string{
		"openclaw/crabbox":                         "openclaw/crabbox",
		"https://github.com/openclaw/crabbox.git":  "openclaw/crabbox",
		"git@github.com:openclaw/crabbox.git":      "openclaw/crabbox",
		"ssh://git@github.com/openclaw/crabbox":    "openclaw/crabbox",
		"https://github.com/openclaw/crabbox/pull": "openclaw/crabbox",
	}
	for input, want := range tests {
		got, err := parseGitHubRepo(input)
		if err != nil {
			t.Fatalf("parseGitHubRepo(%q): %v", input, err)
		}
		if got.Slug() != want {
			t.Fatalf("parseGitHubRepo(%q)=%q want %q", input, got.Slug(), want)
		}
	}
}

func TestGitHubActionsRunnerLabels(t *testing.T) {
	cfg := baseConfig()
	cfg.Profile = "Project Check"
	cfg.Class = "beast"
	cfg.Actions.RunnerLabels = []string{"linux-large", "crabbox"}
	got := githubActionsRunnerLabels(cfg, "cbx_123", []string{"extra"})
	joined := strings.Join(got, ",")
	for _, want := range []string{
		"crabbox",
		"crabbox-cbx-123",
		"crabbox-profile-project-check",
		"crabbox-class-beast",
		"linux-large",
		"extra",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("labels %q missing %q", joined, want)
		}
	}
	if strings.Count(joined, "crabbox") < 1 {
		t.Fatalf("labels should keep crabbox label: %q", joined)
	}
}

func TestGitHubActionsRunnerInstallScriptUsesOfficialRunner(t *testing.T) {
	got := githubActionsRunnerInstallScript("latest", true)
	for _, want := range []string{
		"https://api.github.com/repos/actions/runner/releases/latest",
		"https://github.com/actions/runner/releases/download/",
		"./config.sh --unattended --replace --ephemeral",
		"crabbox-actions-runner.service",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("install script missing %q", want)
		}
	}
}

func TestParseActionsHydrationState(t *testing.T) {
	got := parseActionsHydrationState("WORKSPACE=/home/runner/work/repo/repo\nRUN_ID=123\nREADY_AT=2026-05-01T00:00:00Z\n")
	if got.Workspace != "/home/runner/work/repo/repo" || got.RunID != "123" || got.ReadyAt == "" {
		t.Fatalf("unexpected hydration state: %#v", got)
	}
}

func TestActionsHydrationStatePathSanitizesLease(t *testing.T) {
	got := actionsHydrationStatePath("cbx_123")
	if got != ".crabbox/actions/cbx-123.env" {
		t.Fatalf("state path=%q", got)
	}
}

func TestRemoteClearActionsHydrationStateRemovesReadyAndStop(t *testing.T) {
	got := remoteClearActionsHydrationState("cbx_123")
	for _, want := range []string{
		".crabbox/actions/cbx-123.env",
		".crabbox/actions/cbx-123.stop",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("clear command %q missing %q", got, want)
		}
	}
}
