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

func TestActionsHydrateFieldsIncludesExpectedJob(t *testing.T) {
	got := strings.Join(actionsHydrateFields("cbx_123", "crabbox-cbx-123", "hydrate", 90, []string{"extra=value"}), "\n")
	for _, want := range []string{
		"crabbox_id=cbx_123",
		"crabbox_runner_label=crabbox-cbx-123",
		"crabbox_keep_alive_minutes=90",
		"crabbox_job=hydrate",
		"extra=value",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("hydrate fields missing %q in %q", want, got)
		}
	}
}

func TestActionsHydrateFieldsOmitsEmptyJobForOldWorkflows(t *testing.T) {
	got := strings.Join(actionsHydrateFields("cbx_123", "crabbox-cbx-123", "", 90, nil), "\n")
	if strings.Contains(got, "crabbox_job=") {
		t.Fatalf("hydrate fields should not send undeclared job input to older workflows: %q", got)
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
	got := parseActionsHydrationState("WORKSPACE=/home/runner/work/repo/repo\nRUN_ID=123\nJOB=hydrate\nENV_FILE=/home/runner/.crabbox/actions/cbx-123.env.sh\nSERVICES_FILE=/home/runner/.crabbox/actions/cbx-123.services\nREADY_AT=2026-05-01T00:00:00Z\n")
	if got.Workspace != "/home/runner/work/repo/repo" || got.RunID != "123" || got.Job != "hydrate" || got.EnvFile == "" || got.ServicesFile == "" || got.ReadyAt == "" {
		t.Fatalf("unexpected hydration state: %#v", got)
	}
}

func TestActionsHydrationStatePathMatchesWorkflowInput(t *testing.T) {
	got := actionsHydrationStatePath("cbx_123")
	if got != ".crabbox/actions/cbx_123.env" {
		t.Fatalf("state path=%q", got)
	}
}

func TestRemoteClearActionsHydrationStateRemovesReadyAndStop(t *testing.T) {
	got := remoteClearActionsHydrationState("cbx_123")
	for _, want := range []string{
		".crabbox/actions/cbx_123.env",
		".crabbox/actions/cbx_123.env.sh",
		".crabbox/actions/cbx_123.services",
		".crabbox/actions/cbx_123.stop",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("clear command %q missing %q", got, want)
		}
	}
}

func TestRemoteWriteActionsHydrationStopMatchesWorkflowInput(t *testing.T) {
	got := remoteWriteActionsHydrationStop("cbx_123")
	for _, want := range []string{
		".crabbox/actions",
		".crabbox/actions/cbx_123.stop",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stop command %q missing %q", got, want)
		}
	}
}
