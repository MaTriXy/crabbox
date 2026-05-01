package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	app := App{Stdout: &out, Stderr: &bytes.Buffer{}}
	if err := app.Run(context.Background(), []string{"--version"}); err != nil {
		t.Fatalf("Run(--version) error: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != version {
		t.Fatalf("Run(--version)=%q want %q", got, version)
	}
}

func TestRemoteCommandQuotesWorkdirEnvAndArgs(t *testing.T) {
	got := remoteCommand("/work/crabbox/cbx_1/openclaw", map[string]string{"NODE_OPTIONS": "--max-old-space-size=8192"}, []string{"pnpm", "check:changed"})
	for _, want := range []string{
		"cd '/work/crabbox/cbx_1/openclaw'",
		"NODE_OPTIONS='--max-old-space-size=8192'",
		"bash -lc",
		"'exec \"$@\"' bash 'pnpm' 'check:changed'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("remoteCommand() missing %q in %q", want, got)
		}
	}
}

func TestRemoteShellCommandRunsScript(t *testing.T) {
	got := remoteShellCommand("/work/crabbox/cbx_1/repo", map[string]string{"CI": "1"}, "pnpm install && pnpm test")
	for _, want := range []string{
		"cd '/work/crabbox/cbx_1/repo'",
		"CI='1'",
		"bash -lc 'pnpm install && pnpm test'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("remoteShellCommand() missing %q in %q", want, got)
		}
	}
}

func TestRemoteCommandSourcesActionsEnvFile(t *testing.T) {
	got := remoteCommandWithEnvFile("/home/runner/work/repo/repo", map[string]string{"CI": "1"}, "/home/runner/.crabbox/actions/cbx-123.env.sh", []string{"pnpm", "test"})
	for _, want := range []string{
		"cd '/home/runner/work/repo/repo'",
		"if [ -f '/home/runner/.crabbox/actions/cbx-123.env.sh' ]; then . '/home/runner/.crabbox/actions/cbx-123.env.sh'; fi",
		"CI='1'",
		"'exec \"$@\"' bash 'pnpm' 'test'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("remoteCommandWithEnvFile() missing %q in %q", want, got)
		}
	}
}

func TestShouldUseShellForControlOperators(t *testing.T) {
	if !shouldUseShell([]string{"pnpm", "install", "&&", "pnpm", "test"}) {
		t.Fatal("expected shell mode for && token")
	}
	if !shouldUseShell([]string{"pnpm install && pnpm test"}) {
		t.Fatal("expected shell mode for single shell string")
	}
	if shouldUseShell([]string{"pnpm", "test"}) {
		t.Fatal("plain argv command should not use shell")
	}
}

func TestEnvAllowlist(t *testing.T) {
	if !envAllowed("CUSTOM_TOKEN", []string{"CI", "CUSTOM_*"}) {
		t.Fatal("wildcard env allow failed")
	}
	if envAllowed("PROJECT_TOKEN", []string{"CI", "NODE_OPTIONS"}) {
		t.Fatal("unexpected env forwarding without config")
	}
}

func TestSSHArgsIncludeReliabilityOptions(t *testing.T) {
	t.Setenv("HOME", "/tmp/crabbox-home")
	got := strings.Join(sshArgs(SSHTarget{
		User: "crabbox",
		Host: "203.0.113.10",
		Key:  "/tmp/key",
		Port: "2222",
	}, "true"), "\n")
	for _, want := range []string{
		"ConnectTimeout=10",
		"ConnectionAttempts=3",
		"ServerAliveInterval=15",
		"ServerAliveCountMax=2",
		"UserKnownHostsFile=/tmp/crabbox-home/.ssh/known_hosts",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("sshArgs() missing %q in %q", want, got)
		}
	}
}

func TestSSHPortCandidatesPreferConfiguredPortWithFallback(t *testing.T) {
	tests := map[string][]string{
		"":     {"22"},
		"22":   {"22"},
		"2222": {"2222", "22"},
	}
	for in, want := range tests {
		got := sshPortCandidates(in)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("sshPortCandidates(%q)=%v want %v", in, got, want)
		}
	}
}

func TestIsBootstrapWaitError(t *testing.T) {
	if !isBootstrapWaitError(exit(5, "timed out waiting for SSH on 203.0.113.10 during bootstrap")) {
		t.Fatal("expected SSH timeout to be retryable")
	}
	if isBootstrapWaitError(exit(6, "rsync failed")) {
		t.Fatal("sync failure must not be treated as retryable bootstrap")
	}
}

func TestServerProviderKeyUsesOnlyCrabboxLeaseKeys(t *testing.T) {
	server := Server{Labels: map[string]string{"lease": "cbx_123456abcdef"}}
	if got := serverProviderKey(server); got != "crabbox-cbx-123456abcdef" {
		t.Fatalf("serverProviderKey()=%q", got)
	}
	if !validCrabboxProviderKey("crabbox-cbx-123456abcdef") {
		t.Fatal("expected per-lease provider key to be valid")
	}
	if validCrabboxProviderKey("crabbox-steipete") {
		t.Fatal("shared key must not be treated as per-lease cleanup key")
	}
}

func TestMoveStoredTestboxKeyHandlesCoordinatorRenamedLease(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	oldPath, err := testboxKeyPath("cbx_111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath+".pub", []byte("pub"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := moveStoredTestboxKey("cbx_111111111111", "cbx_222222222222"); err != nil {
		t.Fatal(err)
	}
	newPath, err := testboxKeyPath("cbx_222222222222")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("moved key missing: %v", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old key still exists or unexpected stat error: %v", err)
	}
}

func TestServerTypeForClass(t *testing.T) {
	tests := map[string]string{
		"standard": "ccx33",
		"fast":     "ccx43",
		"large":    "ccx53",
		"beast":    "ccx63",
		"ccx23":    "ccx23",
	}
	for in, want := range tests {
		if got := serverTypeForClass(in); got != want {
			t.Fatalf("serverTypeForClass(%q)=%q want %q", in, got, want)
		}
	}
}

func TestAWSServerTypeForClass(t *testing.T) {
	tests := map[string]string{
		"standard":     "c7a.8xlarge",
		"fast":         "c7a.16xlarge",
		"large":        "c7a.24xlarge",
		"beast":        "c7a.48xlarge",
		"c8a.24xlarge": "c8a.24xlarge",
	}
	for in, want := range tests {
		if got := serverTypeForProviderClass("aws", in); got != want {
			t.Fatalf("serverTypeForProviderClass(%q)=%q want %q", in, got, want)
		}
	}
}
