# Provider Backend Refactor

Read when:

- refactoring provider dispatch, direct lifecycle, or delegated run behavior;
- rebasing the Daytona or Islo provider pull requests;
- adding a new provider backend;
- changing provider config, provider flags, coordinator routing, list/status/stop,
  cleanup, or capability validation.

For step-by-step implementation guidance, read
[Provider Backends](../provider-backends.md). This document captures design
context and migration notes; the authoring guide is the handrail for new code.

## Context

Crabbox has two real execution models.

The first model is SSH lease execution. Hetzner, AWS, and static SSH produce a
machine reachable through SSH. Crabbox owns the workflow: claim, sync, command
wrapping, stdout/stderr streaming, result collection, timing, heartbeat, and
release.

The second model is delegated execution. Blacksmith Testboxes, Daytona `run`,
and Islo own machine setup or file/workspace transport, command execution, and
output streaming. Crabbox keeps provider selection, config, local claims/slugs,
and timing summaries, but it does not rsync into these providers.

Relevant pull requests:

- Daytona provider: https://github.com/openclaw/crabbox/pull/32
- Islo SDK provider: https://github.com/openclaw/crabbox/pull/24
- older Islo CLI provider: https://github.com/openclaw/crabbox/pull/16

SDK/source checks:

- Daytona upstream ships a generated Go API client at
  `github.com/daytonaio/daytona/libs/api-client-go` and a toolbox SDK at
  `github.com/daytonaio/daytona/libs/sdk-go`. Use both through narrow
  Crabbox-owned adapters: the generated client for list/get/start/delete,
  labels, last activity, and SSH access; the SDK/toolbox for sandbox create,
  file upload, and command execution.
- Daytona snapshot creation does not accept CPU/memory/disk resources. Resource
  fields live on image creation. Snapshot-only mode must not expose resource
  flags that become no-ops.
- Daytona JWT auth uses an organization header in the generated client. Require
  `DAYTONA_ORGANIZATION_ID` for JWT auth unless upstream docs prove the selected
  account flow does not need it.
- Islo's Go SDK is young, low-adoption, generated, and has no tagged versions in
  the checked source. It is acceptable behind a narrow Crabbox-owned adapter only
  if the provider is accepted at all.
- Islo's SDK execution stream does not expose a clean typed streaming iterator
  today. Keep the custom SSE consumer from the PR until upstream provides a
  usable stream API.
- https://github.com/openclaw/crabbox/pull/24 superseded
  https://github.com/openclaw/crabbox/pull/16 but was closed for product-fit and
  scope concerns. Rebase it only as a delegated backend, not as an SSH-like
  provider.

The current implementation has provider checks spread through command handlers
and helper paths. More `isDaytonaProvider` and `isIsloProvider` branches would
work short term, but every new provider would touch `run`, `warmup`, `list`,
`status`, `stop`, `cleanup`, config, capability validation, and docs.

The refactor should make providers supply small backends while Crabbox core owns
the workflows.

## Design Principle

Providers do not own commands. Providers configure backends. Core commands own
workflow orchestration.

The command flow should look like this:

```go
backend, err := loadBackend(cfg, runtime)
if err != nil {
	return err
}

switch b := backend.(type) {
case DelegatedRunBackend:
	return b.Run(ctx, runReq)

case SSHLeaseBackend:
	lease, acquired, err := acquireOrResolve(ctx, b, runReq)
	if err != nil {
		return err
	}
	return runOverSSHLease(ctx, b, lease, runReq, acquired)

default:
	return exit(2, "provider=%s does not support run", backend.Spec().Name)
}
```

Provider implementations should not receive `App`. They receive a narrow
runtime and typed request structs.

## Goals

- Keep all current providers working.
- Rebase Daytona as an SSH lease backend.
- Rebase Islo as a delegated run backend.
- Keep Hetzner/AWS broker behavior intact when a coordinator is configured.
- Make coordinator routing a wrapper around SSH lease backends, not provider
  branching inside each command.
- Register built-in provider flags before parsing so provider-specific flags do
  not fail before provider selection.
- Keep built-in providers compiled into the Go binary.
- Avoid Go dynamic plugins.
- Leave an external process plugin protocol as a later extension point.
- Keep provider credentials out of repo config and command arguments.

## Current Implementation State

The first landing implements the provider seam for the existing services:

- `warmup`, `run`, `list`, `status`, `stop`, `cleanup`, lease resolution, and
  best-effort touch now load a backend instead of branching on provider names.
- Built-in providers live under `internal/providers/<name>` and are imported by
  `cmd/crabbox` through `internal/providers/all`.
- Hetzner, AWS, static SSH, and the coordinator wrapper implement
  `SSHLeaseBackend`.
- Blacksmith implements `DelegatedRunBackend` and uses injected
  `CommandRunner` instead of package-level `exec.Command`.
- Command rendering for `list` and `status` is core-owned for both backend
  kinds.
- `App` no longer owns direct Hetzner/AWS/static acquire or resolve helpers.

## Non-Goals

- No runtime-loaded Go `.so` plugins.
- No provider marketplace in this refactor.
- No coordinator support for Daytona or Islo in the first pass.
- No generic remote filesystem abstraction.
- No attempt to make Islo look like SSH unless Islo later ships a stable SSH
  contract.
- No VNC, screenshot, desktop, browser, code portal, or Actions runner support
  for Daytona/Islo unless a provider backend explicitly implements those
  features later.

## Provider And Backend Interfaces

`Provider` is the registration and configuration layer:

```go
type Provider interface {
	Name() string
	Aliases() []string
	Spec() ProviderSpec

	RegisterFlags(fs *flag.FlagSet, defaults Config) any
	ApplyFlags(cfg *Config, fs *flag.FlagSet, values any) error

	Configure(cfg Config, rt Runtime) (Backend, error)
}
```

`Backend` is the configured runtime object:

```go
type Backend interface {
	Spec() ProviderSpec
}
```

Only two backend shapes are needed initially.

### SSH Lease Backend

```go
type SSHLeaseBackend interface {
	Backend

	Acquire(ctx context.Context, req AcquireRequest) (LeaseTarget, error)
	Resolve(ctx context.Context, req ResolveRequest) (LeaseTarget, error)
	List(ctx context.Context, req ListRequest) ([]LeaseView, error)
	ReleaseLease(ctx context.Context, req ReleaseLeaseRequest) error
	Touch(ctx context.Context, req TouchRequest) (Server, error)
}
```

This is for providers that can hand Crabbox an SSH target. Core owns sync and
command execution after acquisition.

### Delegated Run Backend

```go
type DelegatedRunBackend interface {
	Backend

	Warmup(ctx context.Context, req WarmupRequest) error
	Run(ctx context.Context, req RunRequest) (RunResult, error)
	List(ctx context.Context, req ListRequest) ([]LeaseView, error)
	Status(ctx context.Context, req StatusRequest) (statusView, error)
	Stop(ctx context.Context, req StopRequest) error
}
```

This is for providers that own execution. Core does not call SSH, rsync, or
remote command wrapping for these providers. Delegated providers may stream
stdout/stderr during `Run`, but they should not own normal `list` or `status`
rendering when a normalized value can describe the result. If a provider has a
lossy or native-only status shape, keep that loss inside its backend and return
the closest status view instead of printing directly from command code. The
current implementation still uses unexported `statusView`; exporting
`StatusView` is a follow-up before delegated backend implementations can move
fully out of `internal/cli`.

### Optional Backend Interfaces

Cleanup should be optional:

```go
type CleanupBackend interface {
	Backend

	Cleanup(ctx context.Context, req CleanupRequest) error
}
```

Provider pricing can be added later as another optional interface:

```go
type PricingBackend interface {
	Backend

	Price(ctx context.Context, req PriceRequest) (HourlyPrice, error)
}
```

## Runtime

Backends should receive a narrow runtime instead of `App`:

```go
type Runtime struct {
	Stdout io.Writer
	Stderr io.Writer
	Clock  Clock
	HTTP   *http.Client
	Exec   CommandRunner
}

type CommandRunner interface {
	Run(ctx context.Context, req LocalCommandRequest) (LocalCommandResult, error)
}

type LocalCommandRequest struct {
	Name   string
	Args   []string
	Env    []string
	Dir    string
	Stdout io.Writer
	Stderr io.Writer
}

type LocalCommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}
```

Provider modules should not reach into command state, global command handlers,
or `App` methods. If they need a helper, move that helper into a small shared
package or pass it through a request/runtime field.

Tests can then inject writers, clocks, fake HTTP clients, and fake backends
without constructing a full CLI app. `CommandRunner` is the seam for delegated
CLI providers such as Blacksmith so tests do not depend on package-level
`exec.Command` hooks.

## Provider Spec

Provider capabilities should be declarative and typed, not a growing list of
provider-name checks.

```go
type ProviderSpec struct {
	Name        string
	Kind        ProviderKind
	Targets     []TargetSpec
	Features    FeatureSet
	Coordinator CoordinatorMode
}

type ProviderKind string

const (
	ProviderKindSSHLease     ProviderKind = "ssh-lease"
	ProviderKindDelegatedRun ProviderKind = "delegated-run"
)

type CoordinatorMode string

const (
	CoordinatorNever     CoordinatorMode = "never"
	CoordinatorSupported CoordinatorMode = "supported"
)

type TargetSpec struct {
	OS          string
	WindowsMode string
}

type Feature string

const (
	FeatureSSH         Feature = "ssh"
	FeatureCrabboxSync Feature = "crabbox-sync"
	FeatureCleanup     Feature = "cleanup"
	FeatureDesktop     Feature = "desktop"
	FeatureBrowser     Feature = "browser"
	FeatureCode        Feature = "code"
	FeatureTailscale   Feature = "tailscale"
)
```

Do not model Actions runner hydration as an AWS provider feature. That workflow
is core-over-SSH after a Linux lease exists. Validate `--actions-runner` as
"requires `SSHLeaseBackend`, target Linux, and not delegated" unless the provider
later owns a distinct hosted-runner product.

Initial provider matrix:

```text
provider            kind           coordinator  features
hetzner             ssh-lease      supported    ssh, crabbox-sync, cleanup, tailscale
aws                 ssh-lease      supported    ssh, crabbox-sync, cleanup, desktop, browser, code
ssh                 ssh-lease      never        ssh, crabbox-sync, desktop, browser, code
daytona             ssh-lease      never        ssh, crabbox-sync
blacksmith-testbox  delegated-run  never        delegated execution
islo                delegated-run  never        delegated execution
```

Initial target matrix:

```text
hetzner             linux
aws                 linux, windows/normal, windows/wsl2, macos
ssh                 linux, windows/normal, windows/wsl2, macos
daytona             linux
blacksmith-testbox  provider-owned linux
islo                provider-owned linux
```

Capability errors should come from `ProviderSpec` plus provider-specific
validation:

```text
provider=daytona managed provisioning supports target=linux only
desktop/VNC is not supported for provider=islo; islo sandboxes are headless
--actions-runner requires an SSH lease provider with target=linux
```

## Registry

Built-in providers register at init time:

```go
var providerRegistry = map[string]Provider{}

func RegisterProvider(provider Provider) {
	names := append([]string{provider.Name()}, provider.Aliases()...)
	for _, name := range names {
		key := normalizeProviderName(name)
		if key == "" {
			panic("provider name is empty")
		}
		if providerRegistry[key] != nil {
			panic("provider already registered: " + key)
		}
		providerRegistry[key] = provider
	}
}

func ProviderFor(name string) (Provider, error) {
	provider := providerRegistry[normalizeProviderName(name)]
	if provider == nil {
		return nil, exit(2, "unknown provider %q", name)
	}
	return provider, nil
}
```

Canonical provider names:

```text
hetzner
aws
ssh
blacksmith-testbox
daytona
islo
```

Compatibility aliases:

```text
static       -> ssh
static-ssh   -> ssh
blacksmith   -> blacksmith-testbox
```

Docs should use canonical names.

## On-Disk Layout

Use one folder per provider for registration, provider-specific flags, provider
specs, and backend configuration:

```text
internal/providers/all                 # imports every built-in provider
internal/providers/hetzner             # Hetzner provider registration/spec
internal/providers/aws                 # AWS provider registration/spec
internal/providers/ssh                 # static SSH provider registration/spec
internal/providers/blacksmith          # Blacksmith provider registration/spec
internal/cli/provider_backend.go       # core interfaces, registry, requests
internal/cli/providers_common.go       # shared direct SSH backend helpers
internal/cli/provider_aws.go           # AWS SSH lease backend implementation
internal/cli/provider_hetzner.go       # Hetzner SSH lease backend implementation
internal/cli/provider_static.go        # static SSH lease backend implementation
internal/cli/provider_coordinator.go   # brokered coordinator lease backend
internal/cli/provider_blacksmith.go    # delegated Blacksmith backend implementation
internal/cli/hcloud.go                 # Hetzner API client
internal/cli/aws.go                    # AWS API client
internal/cli/static.go                 # static SSH target mapping and flags
internal/cli/blacksmith.go             # Blacksmith args/parsing helpers
```

The first split keeps backend implementations in `internal/cli` because the
existing providers still use broad unexported lifecycle helpers for SSH keys,
claims, labels, slugs, coordinator heartbeats, sync, release, and timing. The
exported contract between provider folders and CLI is deliberately small:
`Provider`, `ProviderSpec`, request/result types, `Runtime`, and one backend
constructor per built-in provider.

Move each backend implementation deeper into `internal/providers/<name>` only
as the required helper surface becomes intentionally exported. New providers
such as Daytona and Islo should start in their own provider folder and avoid
depending on CLI internals that are not part of that exported contract.

## Flag Parsing

Go's `flag` package rejects unknown flags during parse. This means
provider-specific flags must be registered before `flag.Parse`, even though the
selected provider is only known after config and flags are merged.

Use this first-pass strategy for built-in providers:

1. register common command flags;
2. iterate over all registered built-in providers and call `RegisterFlags`;
3. parse once;
4. load config;
5. apply common flags;
6. select `ProviderFor(cfg.Provider)`;
7. apply only the selected provider's parsed flag values;
8. configure the backend.

Example:

```go
providerFlagValues := RegisterAllProviderFlags(fs, defaults)
if err := parseFlags(fs, args); err != nil {
	return err
}

cfg, err := loadConfig()
if err != nil {
	return err
}
applyCommonFlags(&cfg, fs, commonValues)

provider, err := ProviderFor(cfg.Provider)
if err != nil {
	return err
}
if err := ApplySelectedProviderFlags(provider, &cfg, fs, providerFlagValues); err != nil {
	return err
}

backend, err := provider.Configure(cfg, runtime)
```

Flags for non-selected providers are parsed but ignored.

Provider `ApplyFlags` methods must only mutate config for flags that were
actually present in argv, using `flagWasSet` or equivalent. The values passed to
`RegisterFlags` exist so the parser and help text know the flag shape; they must
not overwrite repo config just because every built-in provider flag was
registered up front.

A two-pass parser should only be introduced if external process providers need
to define flags dynamically. In that future design, pass one parses only safe
global selectors such as `--provider` and `--config`, loads provider metadata,
registers provider flags, and pass two parses the original args.

Provider-specific flags:

```text
--blacksmith-org
--blacksmith-workflow
--blacksmith-job
--blacksmith-ref

--daytona-snapshot
--daytona-target
--daytona-user
--daytona-work-root
--daytona-ssh-access-minutes

--islo-image
--islo-workdir
--islo-gateway-profile
```

Avoid exposing provider flags that cannot work. For Daytona, do not expose
CPU/memory/disk overrides while the integration is snapshot-only and Daytona
rejects resource fields with snapshots. Either implement image mode fully or
hide resource overrides.

## Backend Loading

All commands should use the same loading shape:

```go
func loadBackend(cfg Config, rt Runtime) (Backend, error) {
	provider, err := ProviderFor(cfg.Provider)
	if err != nil {
		return nil, err
	}
	backend, err := provider.Configure(cfg, rt)
	if err != nil {
		return nil, err
	}
	if ssh, ok := backend.(SSHLeaseBackend); ok && shouldUseCoordinator(cfg, provider.Spec()) {
		coord, err := newCoordinatorClientForBackend(cfg)
		if err != nil {
			return nil, err
		}
		return NewCoordinatorLeaseBackend(coord, ssh, rt), nil
	}
	return backend, nil
}
```

`Configure` builds direct provider clients and validates provider auth early.
Provider flag registration and `ApplyFlags` happen before this function, during
normal config assembly. `loadBackend` should not know about `flag.FlagSet` or
raw argv.
Examples:

- Hetzner reads `HCLOUD_TOKEN` / `HETZNER_TOKEN`.
- AWS loads AWS SDK config.
- Daytona reads `DAYTONA_API_KEY` or `DAYTONA_JWT_TOKEN`.
- Islo validates `ISLO_API_KEY` before SDK use.
- Blacksmith verifies enough local config to build CLI args.

## Coordinator Wrapper

Coordinator routing should be a wrapper around `SSHLeaseBackend`, not a special
provider path inside every command.

```go
func shouldUseCoordinator(cfg Config, spec ProviderSpec) bool {
	if spec.Coordinator != CoordinatorSupported {
		return false
	}
	return cfg.Coordinator != ""
}
```

Wrapper shape:

```go
type CoordinatorLeaseBackend struct {
	Coord  *CoordinatorClient
	Direct SSHLeaseBackend
	RT     Runtime
}

func NewCoordinatorLeaseBackend(coord *CoordinatorClient, direct SSHLeaseBackend, rt Runtime) SSHLeaseBackend {
	return CoordinatorLeaseBackend{Coord: coord, Direct: direct, RT: rt}
}
```

The wrapper implements `SSHLeaseBackend`:

- `Acquire` calls the coordinator lease API and maps `CoordinatorLease` to
  `LeaseTarget`;
- `Resolve` calls coordinator get/slug lookup and maps to `LeaseTarget`;
- `ReleaseLease` calls coordinator release;
- `Touch` calls heartbeat or idle update paths as appropriate;
- `List` can call coordinator pool/admin routes when available.

In brokered mode, the wrapper owns key creation, coordinator lease creation,
lease lookup, heartbeat, run recorder attachment, and lease release. It must not
fall through to direct Hetzner/AWS acquire, resolve, touch, release, list, or
cleanup calls after the coordinator is selected. The wrapped direct backend
exists only to carry the provider spec and direct-mode implementation for the
non-brokered path.

Brokered list/pool commands still need the existing admin-token enforcement.
Either the command validates that before calling `List`, or
`CoordinatorLeaseBackend.List` returns the same missing-admin-token error. The
wrapper must not silently downgrade brokered pool/list to direct provider list.

Initial coordinator modes:

```text
hetzner             supported
aws                 supported
ssh                 never
daytona             never
blacksmith-testbox  never
islo                never
```

Daytona and Islo can gain broker support later by changing their spec and
implementing Worker-side provider support. That is out of scope for rebasing
the current PRs.

## Request And Result Types

`Provider.Configure` is the only place that should receive full `Config`.
Provider modules should decode their typed config, create provider clients, and
store those on the configured backend. Requests then carry command intent, repo
state, and options. They should not carry `App`, and they should not carry full
`Config` unless a migration step still needs compatibility with old helpers.

```go
type LeaseOptions struct {
	TargetOS        string
	WindowsMode     string
	Class           string
	ServerType      string
	IdleTimeout     time.Duration
	TTL             time.Duration
	Desktop         bool
	Browser         bool
	Code            bool
	ActionsRunner   bool
	Tailscale       TailscaleConfig
	WorkRoot        string
	SSHUser         string
	SSHPort         string
	SSHKey          string
	Sync            SyncConfig
	Results         ResultsConfig
	EnvAllow        []string
}

type AcquireRequest struct {
	Repo    Repo
	Options LeaseOptions
	Keep    bool
	Reclaim bool
}

type ResolveRequest struct {
	Repo    Repo
	Options LeaseOptions
	ID      string
	Reclaim bool
}

type ReleaseLeaseRequest struct {
	Lease LeaseTarget
	Force bool
}

type TouchRequest struct {
	Lease       LeaseTarget
	State       string
	IdleTimeout time.Duration
}

type ListRequest struct {
	Options LeaseOptions
}

type RunRequest struct {
	Repo            Repo
	ID              string
	Options         LeaseOptions
	Keep            bool
	Reclaim         bool
	NoSync          bool
	SyncOnly        bool
	DebugSync       bool
	ShellMode       bool
	ChecksumSync    bool
	ForceSyncLarge  bool
	Command         []string
	TimingJSON      bool
}

type WarmupRequest struct {
	Repo          Repo
	Options       LeaseOptions
	Keep          bool
	Reclaim       bool
	ActionsRunner bool
	TimingJSON    bool
}

type StatusRequest struct {
	Options     LeaseOptions
	ID          string
	Wait        bool
	WaitTimeout time.Duration
}

type StopRequest struct {
	Options LeaseOptions
	ID      string
}

type RunResult struct {
	ExitCode      int
	Command       time.Duration
	Total         time.Duration
	SyncDelegated bool
}
```

Core command code is responsible for converting CLI/config state into
`LeaseOptions` once. Backends should not re-read global command state or decode
raw provider config maps after `Configure`.

`LeaseOptions` is intentionally broad for the migration. Direct provisioning
backends should usually care only about the provisioning subset, while the shared
SSH workflow consumes sync, result, and environment options. After the provider
split lands, consider splitting this into `ProvisionOptions` and `RunOptions`.

`LeaseView` and `StatusView` are command-facing view models. They can wrap or
alias the existing `Server` and `statusView` during migration, but they must
carry redaction metadata for secret-bearing auth. Rendering is core-owned for
both backend kinds: `ListRequest` and `StatusRequest` do not carry JSON or human
format flags because backends return normalized views and core renders them.
`JSONListBackend` is a narrow compatibility escape hatch for existing
script-facing JSON schemas such as coordinator pool machines and Blacksmith
table rows; new providers should not need it.

Delegated providers should reject irrelevant sync options through a shared
helper:

```go
func rejectDelegatedSyncOptions(provider string, req RunRequest) error {
	if req.SyncOnly {
		return exit(2, "provider=%s does not sync local files; --sync-only is not supported", provider)
	}
	if req.ChecksumSync {
		return exit(2, "provider=%s does not sync local files; --checksum is not supported", provider)
	}
	if req.ForceSyncLarge {
		return exit(2, "provider=%s does not sync local files; --force-sync-large is not supported", provider)
	}
	return nil
}
```

## Shared SSH Workflow

`runCommand` should lose the provider lifecycle details and call one shared SSH
workflow:

```go
func runOverSSHLease(
	ctx context.Context,
	backend SSHLeaseBackend,
	lease LeaseTarget,
	req RunRequest,
	acquired bool,
	rt Runtime,
) error
```

This workflow owns:

- local claim/reclaim checks;
- coordinator recorder attachment when the backend is coordinator-wrapped;
- heartbeat/touch lifecycle through `backend.Touch`;
- Actions hydration marker detection;
- sync manifest creation, preflight, git seed, rsync/archive transfer, remote
  prune, and sync finalize;
- POSIX/native Windows/WSL2 command wrapping;
- stdout/stderr streaming and run log buffering;
- JUnit result collection;
- timing summary and timing JSON;
- release through `backend.ReleaseLease` when `acquired && !req.Keep`.

Providers must not copy this workflow. Daytona, Hetzner, AWS, and static SSH all
reuse it.

`ReleaseLease` means "tear down the lease/resource for this specific command or
explicit stop." Background TTL/orphan cleanup is separate and belongs to
`CleanupBackend`. Static SSH can implement `ReleaseLease` as a no-op, but it
must not opt into cleanup.

## Lease Target And SSH Target

Lease backends return:

```go
type LeaseTarget struct {
	Server  Server
	Target  SSHTarget
	LeaseID string
	Options LeaseOptions
}
```

`Server` stays as the neutral provider resource for this refactor:

```go
type Server struct {
	CloudID string
	Provider string
	ID int64
	Name string
	Status string
	Labels map[string]string
	PublicNet struct { IPv4 struct { IP string } }
	ServerType struct { Name string }
}
```

`SSHTarget` needs explicit metadata for secret-bearing auth:

```go
type SSHTarget struct {
	User string
	Host string
	Key string
	Port string
	FallbackPorts []string
	TargetOS string
	WindowsMode string
	ReadyCheck string
	AuthSecret bool
	NetworkKind NetworkMode
}
```

SSH rendering must omit `-i` when `Key == ""`. Human-readable status, list,
timing output, and normal JSON output must redact `User` when `AuthSecret` is
true. The only intended token-revealing surface is an explicit connect action
such as `crabbox ssh --provider daytona --id ...`.

Daytona target example:

```go
SSHTarget{
	User: token,
	Host: parsedHostFromSSHCommand,
	Port: parsedPortFromSSHCommand,
	Key: "",
	TargetOS: "linux",
	ReadyCheck: "command -v git >/dev/null && command -v rsync >/dev/null && command -v tar >/dev/null",
	AuthSecret: true,
	NetworkKind: NetworkPublic,
}
```

Normal output:

```text
ready ssh=<redacted>@<daytona-ssh-host>:<daytona-ssh-port> network=public workroot=/home/daytona/crabbox
```

The actual interactive `crabbox ssh --provider daytona --id ...` command may
print a token-bearing connect command only because the user explicitly asked
for SSH access.

## Provider State Contract

Direct SSH lease providers should map provider resources into `Server` and use
Crabbox labels/tags when the provider supports metadata.

Required labels:

```text
crabbox=true
provider=<provider>
lease=<lease-id>
slug=<friendly-slug>
state=provisioning|leased|ready|running|released|failed
keep=true|false
target=linux|windows|macos
windows_mode=normal|wsl2
server_type=<provider-class-or-instance-type>
created_at=<unix-seconds>
last_touched_at=<unix-seconds>
idle_timeout_secs=<seconds>
ttl_secs=<seconds>
expires_at=<unix-seconds>
```

Current direct providers write Unix seconds. The parser also accepts RFC3339
and RFC3339Nano for compatibility with old or external records. Moving labels
to RFC3339 would be a behavior change and must update Hetzner/AWS tests and
docs together.

Provider-specific labels must be documented:

```text
provider_key=<ssh-key-name>        # Hetzner/AWS direct key cleanup
market=spot|on-demand             # AWS
work_root=<remote-work-root>       # Daytona restore/reuse path
```

If a provider lacks labels/tags, it must implement equivalent lookup and cleanup
semantics before enabling `FeatureCleanup`.

## Config Model

Long term, avoid adding a new top-level `FooConfig` field for every provider.
Use a provider config bag:

```go
type Config struct {
	Provider  string
	Providers map[string]ProviderConfig

	// Compatibility fields kept while migrating existing config.
	Blacksmith BlacksmithConfig
	Static     StaticConfig
}

type ProviderConfig map[string]any
```

YAML:

```yaml
provider: daytona

providers:
  daytona:
    snapshot: crabbox-ready
    target: us
    user: daytona
    workRoot: /home/daytona/crabbox
    sshTokenMinutes: 15

  islo:
    image: docker.io/library/ubuntu:24.04
    workdir: /workspace/crabbox
    gatewayProfile: default
```

Compatibility:

- Keep `blacksmith:` while existing configs migrate.
- Keep `static:` because static SSH is already documented and special.
- Daytona and Islo should prefer `providers.daytona` and `providers.islo`.
- Provider modules should expose typed config accessors so command code never
  decodes raw maps.

Example helper shape:

```go
func DecodeProviderConfig(cfg Config, name string, defaults any, out any) error {
	raw := cfg.Providers[name]
	if raw == nil {
		return copyDefaultProviderConfig(defaults, out)
	}
	return decodeProviderConfig(raw, defaults, out)
}
```

Provider credentials stay in environment or native provider auth stores, not
repo YAML:

```text
HCLOUD_TOKEN
HETZNER_TOKEN
AWS_PROFILE
AWS_REGION
DAYTONA_API_KEY
DAYTONA_JWT_TOKEN
DAYTONA_ORGANIZATION_ID
DAYTONA_API_URL
ISLO_API_KEY
ISLO_BASE_URL
```

## Built-In vs External Plugins

The first refactor keeps providers compiled into the Go binary.

Do not use Go `plugin.Open`. Go plugins require matching Go versions, module
versions, architecture, and build flags. They cannot be unloaded, init code runs
on load, and cross-platform support is poor.

If runtime extension is needed later, use an external process protocol:

```yaml
provider: my-runner
providers:
  my-runner:
    kind: command
    command: crabbox-provider-my-runner
```

The adapter can speak JSON over stdio:

```json
{"method":"spec","params":{}}
{"method":"warmup","params":{"config":{},"keep":true}}
{"method":"run","params":{"id":"...","command":["go","test","./..."]}}
{"method":"status","params":{"id":"...","wait":false}}
{"method":"stop","params":{"id":"..."}}
```

This lets TypeScript or Python SDK adapters exist later without making the core
binary load native plugins.

## Provider Mapping

### Hetzner

Backend: `SSHLeaseBackend`

Spec:

```text
kind=ssh-lease
coordinator=supported
targets=linux
features=ssh, crabbox-sync, cleanup, tailscale
```

Owns direct mode:

- `HCLOUD_TOKEN` / `HETZNER_TOKEN` auth;
- SSH key import/delete;
- server create/list/get/delete;
- labels;
- class fallback;
- direct cleanup.

Reuses core:

- coordinator wrapper when configured;
- SSH sync/run;
- claims;
- status rendering;
- cleanup policy.

### AWS

Backend: `SSHLeaseBackend`

Spec:

```text
kind=ssh-lease
coordinator=supported
targets=linux, windows/normal, windows/wsl2, macos
features=ssh, crabbox-sync, cleanup, desktop, browser, code
```

Owns direct mode:

- AWS SDK config and region selection;
- key pair import/delete;
- AMI resolution;
- security group setup;
- EC2 launch/list/get/terminate;
- Spot/On-Demand fallback;
- Windows/macOS launch options;
- tags;
- direct cleanup.

Reuses core:

- coordinator wrapper when configured;
- SSH sync/run;
- native Windows archive sync and command wrapping;
- claims;
- status rendering;
- cleanup policy.

### Static SSH

Backend: `SSHLeaseBackend`

Spec:

```text
kind=ssh-lease
coordinator=never
targets=linux, windows/normal, windows/wsl2, macos
features=ssh, crabbox-sync, desktop, browser, code
```

Owns:

- static config to `LeaseTarget` mapping;
- static claim behavior;
- no-op release.

Does not support provider cleanup or coordinator.

### Daytona

Backend: hybrid `SSHLeaseBackend` + `DelegatedRunBackend`

Spec:

```text
kind=ssh-lease
coordinator=never
targets=linux
features=ssh, crabbox-sync
```

Owns:

- Daytona generated Go API client auth and organization header;
- Daytona SDK/toolbox auth;
- sandbox create/list/get/start/stop/delete;
- labels and last-activity touch;
- SSH access token minting;
- toolbox archive upload and command execution for `run`;
- Daytona sandbox to `Server` mapping;
- secret SSH user and public relay target metadata.

Reuses core:

- sync manifest and guardrails;
- claims;
- status rendering;
- explicit release/stop.

Initial constraints:

- Linux only.
- No coordinator.
- No Tailscale.
- No VNC/screenshot/desktop/browser/code portal.
- Actions runner hydration is not supported for Daytona warmup.
- Snapshot mode only unless image mode is implemented fully.

Rebase notes for https://github.com/openclaw/crabbox/pull/32:

- Implement `Provider.Configure` returning a Daytona backend that supports
  delegated `run` plus explicit SSH access.
- Use Daytona's generated Go API client and SDK/toolbox; do not duplicate REST
  plumbing in Crabbox.
- Keep start-before-SSH for stopped sandboxes.
- Require `DAYTONA_ORGANIZATION_ID` when JWT auth is used unless Daytona docs
  prove it is optional for the account shape.
- Do not expose CPU/memory/disk flags while snapshot mode makes them unusable.
- Keep token redaction tests.

### Blacksmith Testbox

Backend: `DelegatedRunBackend`

Spec:

```text
kind=delegated-run
coordinator=never
targets=provider-owned linux
features=delegated execution
```

Owns:

- Blacksmith CLI command construction;
- warmup/run/list/status/stop;
- Testbox SSH key storage for Blacksmith CLI, through injected filesystem/runtime
  helpers;
- provider-specific claim ID resolution;
- delegated timing summaries.

Does not support Crabbox rsync, `--sync-only`, VNC/screenshot/desktop through
Crabbox, or coordinator.

### Islo

Backend: `DelegatedRunBackend`

Spec:

```text
kind=delegated-run
coordinator=never
targets=provider-owned linux
features=delegated execution
```

Owns:

- SDK auth and token refresh;
- sandbox create/list/get/delete;
- command execution through provider API;
- SSE parsing for live stdout/stderr;
- Islo lease ID and sandbox name mapping;
- delegated timing summaries.

Does not support Crabbox rsync, `--sync-only`, `--checksum`,
`--force-sync-large`, VNC/screenshot/desktop/browser through Crabbox, Actions
runner, or coordinator.

Rebase notes for https://github.com/openclaw/crabbox/pull/24:

- Implement `Provider.Configure` returning an Islo `DelegatedRunBackend`.
- Keep the small Go SDK dependency if the provider is accepted.
- Keep the custom SSE consumer; the SDK stream method does not expose a clean
  streaming API today.
- Validate `ISLO_API_KEY` before SDK calls.
- Keep `ISLO_BASE_URL` as the only base URL override.
- Keep delegated option rejection tests.

## Migration Plan

### Phase 1: Registry And Specs

- Add provider registry.
- Add `Provider`, `Backend`, `ProviderSpec`, and feature/target types.
- Register existing providers as built-ins.
- Keep current command behavior.
- Register all built-in provider flags before `flag.Parse`.

Expected behavior change: none.

Status: implemented for existing built-in providers in
`internal/providers/<name>`.

### Phase 2: Backend Loading

- Add `Runtime`.
- Add `Provider.Configure`.
- Add `loadBackend`.
- Add fake backend tests for command dispatch.
- Keep old provider helper functions temporarily.

Expected behavior change: none.

Status: implemented. `loadBackend(cfg Config, rt Runtime)` intentionally does
not accept `flag.FlagSet` or raw command args.

### Phase 3: Coordinator Wrapper

- Add `CoordinatorLeaseBackend`.
- Wrap Hetzner/AWS SSH lease backends when coordinator is configured.
- Prove logged-in/configured users still go through the broker.
- Keep direct Hetzner/AWS when coordinator is disabled.

Expected behavior change: none.

Status: implemented for Hetzner/AWS coordinator-backed leases.

### Phase 4: Extract Shared SSH Workflow

- Extract `runOverSSHLease`.
- Route Hetzner, AWS, and static SSH through `SSHLeaseBackend`.
- Preserve heartbeat, recorder, release, sync, Windows archive sync, and JUnit
  behavior.
- Add fake SSH backend tests before rebasing any new provider. These tests should
  prove acquire, resolve-by-id, claim/reclaim, sync-only, heartbeat/touch,
  timing JSON, release-on-non-keep, and run-recorder behavior without hitting a
  real provider.

Expected behavior change: none.

Status: implemented for existing SSH providers. New provider PRs should add fake
backend tests before adding live-only coverage.

### Phase 5: Convert Delegated Providers

- Move Blacksmith into a `DelegatedRunBackend`.
- Centralize delegated sync-option rejection.
- Dispatch `warmup`, `run`, `list`, `status`, and `stop` through backend shape.

Expected behavior change: none.

Status: implemented for Blacksmith Testbox.

### Phase 6: Provider Config Bag

- Add `providers:` YAML parsing.
- Add typed provider config decoders.
- Keep existing `blacksmith:` and `static:` compatibility.
- Prefer `providers.<name>` for new providers and docs.

Expected behavior change: none for existing configs.

### Phase 7: Rebase Daytona

- Rebase https://github.com/openclaw/crabbox/pull/32 onto a hybrid Daytona
  backend: delegated SDK/toolbox `run`, explicit SSH access for `ssh`.
- Keep Daytona SDK access isolated behind the backend adapter.
- Add tests for acquire/resolve/list/release/touch plus delegated backend
  selection.
- Add redaction tests for secret SSH user output.
- Add live smoke behind explicit env gates only.

Expected behavior change: new provider.

### Phase 8: Rebase Islo

- Rebase https://github.com/openclaw/crabbox/pull/24 onto
  `DelegatedRunBackend`.
- Keep SDK seam injectable.
- Keep SSE parser tests.
- Add delegated option rejection tests.
- Add live smoke behind explicit env gates only.

Expected behavior change: new provider if product decision is yes.

### Phase 9: Remove Compatibility Branches

- Remove direct command references to `isBlacksmithProvider`,
  `isDaytonaProvider`, and `isIsloProvider`.
- Replace remaining static checks with canonical provider/spec checks where
  practical.
- Update `docs/source-map.md` and provider feature docs.

Expected behavior change: none.

## Tests

Registry and flag tests:

- canonical lookup;
- alias lookup;
- duplicate registration panic;
- unknown provider error;
- provider help string includes built-ins;
- built-in provider flags are accepted before provider selection;
- non-selected provider flags parse but are ignored.

Spec and capability tests:

- target OS and Windows mode validation per provider;
- unsupported desktop/browser/code provider features and Actions runner
  capability errors;
- coordinator wrapper selected for Hetzner/AWS when configured;
- direct backend selected for static, Daytona, and coordinator-disabled
  Hetzner/AWS.

SSH workflow tests:

- fake SSH backend acquire path enters shared sync/run;
- fake SSH backend resolve path enters shared sync/run;
- touch transitions go through backend;
- release happens on acquired non-keep lease;
- no provider-specific command branch is needed for fake SSH backend.

Delegated backend tests:

- fake delegated backend receives warmup/run/list/status/stop requests;
- delegated sync flags are rejected;
- nonzero exit code propagates;
- Blacksmith command execution goes through injected `CommandRunner`, not
  package-level `exec.Command`;
- Blacksmith Testbox SSH key storage goes through injected filesystem/runtime
  helpers where practical.

Daytona tests:

- auth env validation;
- organization header behavior;
- create body shape;
- labels body shape;
- snapshot mode omits unusable resource overrides;
- stopped sandbox starts before SSH target creation;
- SSH target parses the API-returned `sshCommand`, uses empty key, secret user,
  public network, and ready check;
- list/status/timing output, including JSON, redacts token-bearing user;
- release removes local claim.

Islo tests:

- SDK factory rejects missing `ISLO_API_KEY`;
- SDK client maps create/get/list/delete;
- SSE parser handles stdout/stderr/exit events;
- run streams output and propagates exit code;
- status wait polls and times out;
- stop removes local claim.

Docs tests:

- provider docs link from `docs/features/providers.md`;
- `docs/source-map.md` lists provider implementation files;
- command docs mention provider list consistently.

## Acceptance Criteria

- `go test ./...` passes.
- Existing providers keep working:
  - `crabbox warmup --provider hetzner`
  - `crabbox run --provider aws`
  - `crabbox run --provider ssh`
  - `crabbox run --provider blacksmith-testbox`
- A fake SSH lease backend can be tested without editing command handlers.
- A fake delegated backend can be tested without editing command handlers.
- Hetzner/AWS still use the coordinator when configured.
- Daytona can be rebased by implementing the hybrid backend.
- Islo can be rebased by implementing `DelegatedRunBackend`.
- No new provider requires touching the main command flow unless it adds a new
  top-level Crabbox feature.
- Normal list/status/timing output, including JSON, never prints secret SSH users
  or provider API credentials.

## Open Questions

- Should `Server` become `Machine` after providers no longer all create
  servers?
- Should `providers.<name>` become the only provider config namespace in a
  future major release?
- Should external command providers use a small Crabbox JSON protocol or MCP?
  The smaller JSON protocol is preferred for now.
- Should Daytona support image mode and resource overrides, or stay snapshot
  only?
- Should Islo be accepted as a built-in provider at all, given the product-fit
  concerns from the closed PRs?
