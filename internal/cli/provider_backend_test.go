package cli

import (
	"context"
	"io"
	"strings"
	"testing"
)

type recordingCommandRunner struct {
	calls  []LocalCommandRequest
	result LocalCommandResult
	err    error
}

func (r *recordingCommandRunner) Run(_ context.Context, req LocalCommandRequest) (LocalCommandResult, error) {
	r.calls = append(r.calls, req)
	return r.result, r.err
}

func testRuntimeWithRunner(r CommandRunner) Runtime {
	return Runtime{Stdout: io.Discard, Stderr: io.Discard, Clock: realClock{}, Exec: r}
}

func TestProviderRegistryCanonicalAndAliases(t *testing.T) {
	for _, name := range []string{"hetzner", "aws", "ssh", "static", "static-ssh", "blacksmith", "blacksmith-testbox", "daytona", "islo"} {
		if _, err := ProviderFor(name); err != nil {
			t.Fatalf("ProviderFor(%q): %v", name, err)
		}
	}
	if _, err := ProviderFor("missing"); err == nil {
		t.Fatal("expected missing provider to fail")
	}
}

func TestLoadBackendWrapsCoordinatorOnlyForSupportedSSHProviders(t *testing.T) {
	cfg := baseConfig()
	cfg.Provider = "aws"
	cfg.Coordinator = "https://coordinator.example"
	backend, err := loadBackend(cfg, testRuntimeWithRunner(&recordingCommandRunner{}))
	if err != nil {
		t.Fatalf("load aws coordinator backend: %v", err)
	}
	if _, ok := backend.(*coordinatorLeaseBackend); !ok {
		t.Fatalf("backend=%T, want coordinatorLeaseBackend", backend)
	}

	cfg.Provider = "ssh"
	backend, err = loadBackend(cfg, testRuntimeWithRunner(&recordingCommandRunner{}))
	if err != nil {
		t.Fatalf("load static ssh backend: %v", err)
	}
	if _, ok := backend.(*coordinatorLeaseBackend); ok {
		t.Fatalf("static ssh unexpectedly used coordinator wrapper")
	}

	cfg.Provider = "blacksmith-testbox"
	backend, err = loadBackend(cfg, testRuntimeWithRunner(&recordingCommandRunner{}))
	if err != nil {
		t.Fatalf("load blacksmith backend: %v", err)
	}
	if _, ok := backend.(DelegatedRunBackend); !ok {
		t.Fatalf("backend=%T, want delegated run backend", backend)
	}
}

func TestLeaseCreateFlagsApplySelectedProviderFlags(t *testing.T) {
	defaults := baseConfig()
	fs := newFlagSet("test", io.Discard)
	values := registerLeaseCreateFlags(fs, defaults)
	if err := parseFlags(fs, []string{
		"--provider", "blacksmith-testbox",
		"--blacksmith-org", "openclaw",
		"--blacksmith-workflow", ".github/workflows/testbox.yml",
		"--blacksmith-job", "test",
		"--blacksmith-ref", "feature",
	}); err != nil {
		t.Fatal(err)
	}
	cfg := baseConfig()
	if err := applyLeaseCreateFlags(&cfg, fs, values); err != nil {
		t.Fatal(err)
	}
	if cfg.Blacksmith.Org != "openclaw" || cfg.Blacksmith.Workflow != ".github/workflows/testbox.yml" || cfg.Blacksmith.Job != "test" || cfg.Blacksmith.Ref != "feature" {
		t.Fatalf("blacksmith flags not applied through provider registry: %#v", cfg.Blacksmith)
	}
}

func TestValidateRequestedCapabilitiesUsesProviderSpec(t *testing.T) {
	cfg := baseConfig()
	cfg.Provider = "blacksmith-testbox"
	cfg.Desktop = true
	if err := validateRequestedCapabilities(cfg); err == nil {
		t.Fatal("expected blacksmith desktop capability rejection")
	}

	cfg = baseConfig()
	cfg.Provider = "hetzner"
	cfg.Desktop = true
	if err := validateRequestedCapabilities(cfg); err != nil {
		t.Fatalf("hetzner desktop capability rejected: %v", err)
	}
}

func TestBlacksmithBackendUsesInjectedCommandRunnerForListAndStatus(t *testing.T) {
	runner := &recordingCommandRunner{
		result: LocalCommandResult{
			Stdout: "tbx_123 ready openclaw .github/workflows/testbox.yml test main 2026-05-06T00:00:00Z\n",
		},
	}
	cfg := baseConfig()
	cfg.Provider = "blacksmith-testbox"
	cfg.Blacksmith.Workflow = ".github/workflows/testbox.yml"
	cfg.Blacksmith.Job = "test"
	cfg.Blacksmith.Ref = "main"
	backend, err := loadBackend(cfg, testRuntimeWithRunner(runner))
	if err != nil {
		t.Fatalf("load blacksmith backend: %v", err)
	}
	delegated := backend.(DelegatedRunBackend)
	servers, err := delegated.List(context.Background(), ListRequest{Options: leaseOptionsFromConfig(cfg)})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(servers) != 1 || servers[0].CloudID != "tbx_123" {
		t.Fatalf("servers=%#v", servers)
	}
	state, err := delegated.Status(context.Background(), StatusRequest{Options: leaseOptionsFromConfig(cfg), ID: "tbx_123"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !state.Ready || state.ID != "tbx_123" {
		t.Fatalf("state=%#v", state)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("runner calls=%d, want 2", len(runner.calls))
	}
	for _, call := range runner.calls {
		if call.Name != "blacksmith" {
			t.Fatalf("command name=%q", call.Name)
		}
	}
}

func TestProviderFlagsApplyDaytonaAndIsloWithoutCoreEdits(t *testing.T) {
	defaults := baseConfig()
	fs := newFlagSet("test", io.Discard)
	provider := fs.String("provider", defaults.Provider, "")
	values := registerProviderFlags(fs, defaults)
	if err := parseFlags(fs, []string{
		"--provider", "daytona",
		"--daytona-snapshot", "snap-crabbox",
		"--daytona-target", "us",
		"--daytona-work-root", "/home/daytona/work",
	}); err != nil {
		t.Fatal(err)
	}
	cfg := defaults
	cfg.Provider = *provider
	if err := applyProviderFlags(&cfg, fs, values); err != nil {
		t.Fatal(err)
	}
	if cfg.Daytona.Snapshot != "snap-crabbox" || cfg.Daytona.Target != "us" || cfg.Daytona.WorkRoot != "/home/daytona/work" {
		t.Fatalf("daytona flags not applied: %#v", cfg.Daytona)
	}

	fs = newFlagSet("test", io.Discard)
	provider = fs.String("provider", defaults.Provider, "")
	values = registerProviderFlags(fs, defaults)
	if err := parseFlags(fs, []string{
		"--provider", "islo",
		"--islo-image", "ubuntu:24.04",
		"--islo-vcpus", "4",
		"--islo-memory-mb", "8192",
	}); err != nil {
		t.Fatal(err)
	}
	cfg = defaults
	cfg.Provider = *provider
	if err := applyProviderFlags(&cfg, fs, values); err != nil {
		t.Fatal(err)
	}
	if cfg.Islo.Image != "ubuntu:24.04" || cfg.Islo.VCPUs != 4 || cfg.Islo.MemoryMB != 8192 {
		t.Fatalf("islo flags not applied: %#v", cfg.Islo)
	}
}

func TestDaytonaAuthRequiresOrganizationForJWT(t *testing.T) {
	cfg := baseConfig()
	cfg.Provider = daytonaProvider
	cfg.Daytona.APIKey = ""
	cfg.Daytona.JWTToken = "jwt"
	cfg.Daytona.OrganizationID = ""
	_, err := newDaytonaClient(cfg, Runtime{})
	if err == nil || !strings.Contains(err.Error(), "DAYTONA_ORGANIZATION_ID") {
		t.Fatalf("err=%v, want organization requirement", err)
	}
}

func TestDaytonaSSHTargetUsesReturnedSSHCommand(t *testing.T) {
	cfg := baseConfig()
	cfg.Daytona.SSHGatewayHost = "fallback.example"
	target, err := daytonaSSHTargetFromAccess(cfg, daytonaSSHAccess{
		Token:   "tok_live_secret",
		Command: "ssh -p 2222 tok_live_secret@region-ssh.example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if target.User != "tok_live_secret" || target.Host != "region-ssh.example.com" || target.Port != "2222" {
		t.Fatalf("target=%#v", target)
	}
	if target.Key != "" || !target.AuthSecret || target.NetworkKind != NetworkPublic {
		t.Fatalf("target auth/network=%#v", target)
	}
}

func TestDaytonaSSHTargetFallsBackWhenCommandMissing(t *testing.T) {
	cfg := baseConfig()
	cfg.Daytona.SSHGatewayHost = "fallback.example"
	target, err := daytonaSSHTargetFromAccess(cfg, daytonaSSHAccess{Token: "tok_live_secret"})
	if err != nil {
		t.Fatal(err)
	}
	if target.User != "tok_live_secret" || target.Host != "fallback.example" || target.Port != "22" {
		t.Fatalf("target=%#v", target)
	}
}

func TestDaytonaBackendIsHybridSDKRunAndSSHAccess(t *testing.T) {
	backend := NewDaytonaLeaseBackend(ProviderSpec{Name: daytonaProvider}, baseConfig(), Runtime{})
	if _, ok := backend.(DelegatedRunBackend); !ok {
		t.Fatal("daytona should use delegated SDK run path")
	}
	if _, ok := backend.(SSHLeaseBackend); !ok {
		t.Fatal("daytona should still expose explicit SSH access")
	}
}

func TestDaytonaCommandString(t *testing.T) {
	if got := daytonaCommandString([]string{"go", "test", "./..."}, false); got != "'go' 'test' './...'" {
		t.Fatalf("command=%q", got)
	}
	if got := daytonaCommandString([]string{"FOO=bar", "go", "test"}, false); !strings.Contains(got, "FOO=") || !strings.Contains(got, "go") {
		t.Fatalf("shell command=%q", got)
	}
	if got := daytonaCommandString([]string{"echo hello && pwd"}, true); got != "echo hello && pwd" {
		t.Fatalf("shell mode=%q", got)
	}
}

func TestRedactedSSHUserOnlyForDaytona(t *testing.T) {
	target := SSHTarget{User: "tok_live_secret"}
	if got := redactedSSHUser(Config{Provider: "hetzner"}, Server{Provider: "hetzner"}, target); got != target.User {
		t.Fatalf("redactedSSHUser hetzner=%q", got)
	}
	if got := redactedSSHUser(Config{Provider: "hetzner"}, Server{Provider: "hetzner"}, SSHTarget{User: "secret", AuthSecret: true}); got != daytonaTokenRedacted {
		t.Fatalf("redactedSSHUser auth secret=%q", got)
	}
	if got := redactedSSHUser(Config{Provider: daytonaProvider}, Server{}, target); got != daytonaTokenRedacted {
		t.Fatalf("redactedSSHUser daytona=%q", got)
	}
}

func TestBlacksmithBackendListJSONKeepsParsedTableShape(t *testing.T) {
	runner := &recordingCommandRunner{
		result: LocalCommandResult{
			Stdout: "tbx_123 ready openclaw .github/workflows/testbox.yml test main 2026-05-06T00:00:00Z\n",
		},
	}
	cfg := baseConfig()
	cfg.Provider = "blacksmith-testbox"
	backend, err := loadBackend(cfg, testRuntimeWithRunner(runner))
	if err != nil {
		t.Fatalf("load blacksmith backend: %v", err)
	}
	jsonBackend, ok := backend.(JSONListBackend)
	if !ok {
		t.Fatalf("backend=%T, want JSONListBackend", backend)
	}
	view, err := jsonBackend.ListJSON(context.Background(), ListRequest{Options: leaseOptionsFromConfig(cfg)})
	if err != nil {
		t.Fatalf("list json: %v", err)
	}
	items, ok := view.([]blacksmithListItem)
	if !ok {
		t.Fatalf("view=%T, want []blacksmithListItem", view)
	}
	if len(items) != 1 || items[0].ID != "tbx_123" || items[0].Repo != "openclaw" {
		t.Fatalf("items=%#v", items)
	}
}
