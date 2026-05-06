package cli

import (
	"context"
	"io"
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
	for _, name := range []string{"hetzner", "aws", "ssh", "static", "static-ssh", "blacksmith", "blacksmith-testbox"} {
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
