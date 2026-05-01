# 🦀 Crabbox

**Warm a box, sync the diff, run the suite.**

Crabbox is an open source remote testbox runner for maintainers and agents. It gives a fast local loop on owned cloud capacity: provision or reuse a warm Linux box, sync the current dirty checkout, run a command remotely, stream output, and clean up.

The current implementation is a Go CLI plus a Cloudflare Worker/Durable Object coordinator. The CLI uses the coordinator for brokered Hetzner or AWS EC2 Spot leases, with direct provider calls kept as a debug fallback.

Documentation lives in [`docs/`](docs/README.md). Start with [How Crabbox Works](docs/how-it-works.md) for the end-to-end mental model, and use [Source Map](docs/source-map.md) to trace docs back to implementation files. The GitHub Pages site is generated from those Markdown files with a small dependency-free builder:

```sh
node scripts/build-docs-site.mjs
open dist/docs-site/index.html
```

## Install

Latest release: `0.1.0`.

```sh
brew install openclaw/tap/crabbox
```

Verify the installed binary:

```sh
crabbox --version
```

Without Homebrew, download the matching archive from the `v0.1.0` release on GitHub:

```text
https://github.com/openclaw/crabbox/releases/tag/v0.1.0
```

## How It Works

Crabbox has a small control plane and a simple data plane:

```text
developer laptop
  crabbox CLI
    |
    | HTTPS JSON API, bearer auth
    v
Cloudflare Worker
  Fleet Durable Object
    |
    | provider API
    v
Hetzner server or AWS EC2 Spot instance

developer laptop
  |
  | rsync + SSH
  v
leased runner
```

The **CLI** is the user-facing tool. It loads config from `~/.config/crabbox/config.yaml`, repo-local `crabbox.yaml` or `.crabbox.yaml`, creates a per-lease SSH key, asks the broker for a lease, waits for SSH, seeds remote Git when possible, builds a Git file-list sync manifest, skips sync when the local/remote fingerprint matches, rsyncs the current checkout, runs the requested command, streams output, and releases the lease unless `--keep` is set. SSH prefers the configured port and can fall back to port 22 during bootstrap.

The **broker** is the Cloudflare Worker at `crabbox-coordinator.steipete.workers.dev`. It authenticates requests with `CRABBOX_SHARED_TOKEN`, routes all fleet operations through a single Durable Object, and owns cloud-provider credentials. Local machines do not need AWS or Hetzner API keys for the normal path.

The **Fleet Durable Object** is the serialized scheduler and lease store. It creates lease IDs, records owner/profile/class/provider metadata, tracks expiry, and has an alarm that expires stale leases. Release and expiry both call the provider delete path for non-kept machines.

The **provider layer** provisions capacity:

- Hetzner: imports or reuses the SSH key, creates a server, applies Crabbox labels, and falls back across configured server types when quota or capacity rejects a request.
- AWS: signs EC2 Query API calls inside the Worker, imports or reuses the SSH key pair, creates or reuses the `crabbox-runners` security group, launches one-time Spot instances, tags instances/volumes/Spot requests, and falls back across broad C/M/R instance families. Direct AWS mode can use Spot placement scores across configured regions before provisioning.

The **runner** is just an Ubuntu machine bootstrapped by cloud-init. Bootstrap creates the `crabbox` user, enables SSH on port `2222`, installs only Crabbox plumbing (`curl`, Git, rsync, jq, OpenSSH), and prepares `/work/crabbox` plus cache directories. Project runtimes such as Go, Node, pnpm, Docker, services, and secrets belong in the repository's GitHub Actions hydration, devcontainer, Nix, mise/asdf, or setup scripts. The runner does not need broker credentials.

The normal lifecycle is:

1. `crabbox run --class standard -- <command>` loads local config.
2. CLI sends `POST /v1/leases` with provider, class, TTL, idle timeout, slug, SSH public key, and bootstrap options.
3. Worker creates a Hetzner server or AWS Spot instance and stores the lease metadata, including `lastTouchedAt` and idle expiry.
4. CLI waits for `crabbox-ready` over SSH.
5. CLI seeds remote Git when possible, then rsyncs tracked plus nonignored untracked files into `/work/crabbox/<lease>/<repo>`.
6. CLI records sync fingerprints, enforces sync size/time guardrails, runs sync sanity checks, and hydrates configured base-ref history.
7. CLI runs the command over SSH and returns the remote exit code.
8. CLI releases the lease unless it was kept; kept leases still auto-release after idle timeout.

The GitHub Actions hydration lifecycle reuses the same machines, but lets the repository's workflow define setup:

1. `crabbox warmup` leases a reusable box and prints both a stable `cbx_...` ID and a friendly slug.
2. `crabbox actions hydrate --id blue-lobster` registers that box as an ephemeral GitHub Actions runner, dispatches the configured workflow, and waits for the workflow to write a ready marker.
3. The workflow runs normal Actions steps such as checkout, dependency install, cache/service setup, and secret-backed environment hydration.
4. `crabbox run --id blue-lobster -- <command>` syncs the local dirty checkout into the hydrated `$GITHUB_WORKSPACE`, sources the workflow's non-secret env handoff, and runs commands there.

Crabbox does not parse or reimplement GitHub Actions YAML. The project-owned workflow decides what to install and when the machine is ready. GitHub secrets and OIDC request tokens remain workflow-step scoped unless that workflow intentionally persists its own short-lived handoff.

Direct provider mode still exists for debugging. If no broker is configured, `--provider aws` uses the local AWS SDK credential chain and `--provider hetzner` uses `HCLOUD_TOKEN` or `HETZNER_TOKEN`. The brokered path is the default operational model.

## Status

Working today:

- [`crabbox doctor`](docs/commands/doctor.md)
- [`crabbox login`](docs/commands/login.md)
- [`crabbox logout`](docs/commands/logout.md)
- [`crabbox whoami`](docs/commands/whoami.md)
- [`crabbox init`](docs/commands/init.md)
- [`crabbox warmup`](docs/commands/warmup.md)
- [`crabbox run`](docs/commands/run.md)
- [`crabbox sync-plan`](docs/commands/sync-plan.md)
- [`crabbox history`](docs/commands/history.md)
- [`crabbox logs`](docs/commands/logs.md)
- [`crabbox results`](docs/commands/results.md)
- [`crabbox cache`](docs/commands/cache.md)
- [`crabbox status`](docs/commands/status.md)
- [`crabbox list`](docs/commands/list.md)
- [`crabbox usage`](docs/commands/usage.md)
- [`crabbox admin`](docs/commands/admin.md)
- [`crabbox ssh`](docs/commands/ssh.md)
- [`crabbox inspect`](docs/commands/inspect.md)
- [`crabbox stop`](docs/commands/stop.md)
- [`crabbox actions`](docs/commands/actions.md)
- [`crabbox pool list`](docs/commands/list.md)
- [`crabbox machine cleanup`](docs/commands/cleanup.md)
- [`crabbox cleanup`](docs/commands/cleanup.md)
- [Cloudflare Worker coordinator on Workers/Durable Objects](docs/features/coordinator.md)
- [bearer-token coordinator auth for automation](docs/features/broker-auth-routing.md)
- [Cloudflare route for `crabbox.clawd.bot/*`](docs/features/broker-auth-routing.md)
- [Hetzner server provisioning with class fallback](docs/features/providers.md)
- [AWS EC2 Spot provisioning with class fallback](docs/features/providers.md)
- [minimal cloud-init bootstrap for SSH, Git, rsync, and work directories](docs/features/runner-bootstrap.md)
- [Git file-list rsync overlay of tracked and nonignored local files](docs/features/sync.md)
- [sync fingerprint skip for no-change hot runs](docs/features/sync.md)
- [per-lease SSH keys under the Crabbox config directory](docs/features/ssh-keys.md)
- [coordinator cost guardrails and monthly usage summaries](docs/features/cost-usage.md)
- [coordinator run history and retained run-log tails](docs/features/history-logs.md)
- [JUnit test result summaries for recorded runs](docs/features/test-results.md)
- [explicit warm-box cache controls](docs/features/cache.md)
- [operator login, identity, and admin lease controls](docs/features/auth-admin.md)
- [provider-backed price estimates with static fallback rates](docs/features/cost-usage.md)
- [sync sanity checks for mass tracked deletions](docs/features/sync.md)
- [shallow Git hydration for configured base-ref detection](docs/features/sync.md)
- [GitHub Actions-backed hydration into project-defined runner workspaces](docs/features/actions-hydration.md)
- [SSH execution on port `2222`](docs/features/runner-bootstrap.md)

Not yet done:

- untrusted multi-tenant isolation

## Quick Start

Prerequisites:

- `git`, `ssh`, `ssh-keygen`, `rsync`, and `curl`
- broker config in `~/.config/crabbox/config.yaml` or `~/Library/Application Support/crabbox/config.yaml` on macOS

Configure the deployed broker:

```sh
printf '%s' "$CRABBOX_COORDINATOR_TOKEN" | \
  crabbox config set-broker \
    --url https://crabbox-coordinator.steipete.workers.dev \
    --provider aws \
    --token-stdin
```

Check local prerequisites and broker access:

```sh
crabbox doctor
```

Inspect broker config:

```sh
crabbox config show
```

Onboard a repo for Crabbox:

```sh
crabbox init
```

Warm a reusable testbox:

```sh
crabbox warmup --profile project-check --class beast
```

Hydrate that box through the repo's GitHub Actions setup, then run local tests inside the hydrated workspace:

```sh
crabbox actions hydrate --id blue-lobster
CI=1 crabbox run --id blue-lobster -- pnpm test:changed:max
```

Use AWS EC2 Spot through the broker:

```sh
crabbox warmup --class beast
```

Run a command on an existing lease:

```sh
CI=1 crabbox run --id blue-lobster -- pnpm test:changed:max
```

Inspect and connect:

```sh
crabbox status --id blue-lobster
crabbox ssh --id blue-lobster
crabbox inspect --id blue-lobster --json
```

Inspect usage and estimated cost:

```sh
crabbox usage
crabbox usage --scope org --org openclaw
crabbox usage --scope all --json
```

`crabbox usage` reads coordinator history, so it requires a configured broker. Cost is an estimate for compute leases, not a provider invoice: the coordinator prefers explicit `CRABBOX_COST_RATES_JSON` overrides, then provider pricing from AWS Spot history or Hetzner server-type prices, then built-in fallback rates. Full reference: [docs/commands/usage.md](docs/commands/usage.md).

Use the OpenClaw plugin when an agent should drive Crabbox through OpenClaw tools instead of shelling out manually. The repository root is also a native OpenClaw plugin package; install it from this repo or from a packaged release, then use the `crabbox_run`, `crabbox_warmup`, `crabbox_status`, `crabbox_list`, and `crabbox_stop` tools.

Stop a kept server:

```sh
crabbox stop blue-lobster
```

Print the CLI version:

```sh
crabbox --version
```

## Machine Classes

`beast` is the default. Hetzner uses dedicated-server classes:

```text
standard  ccx33, cpx62, cx53
fast      ccx43, cpx62, cx53
large     ccx53, ccx43, cpx62, cx53
beast     ccx63, ccx53, ccx43, cpx62, cx53
```

During verification, Hetzner rejected `ccx63`, `ccx53`, and `ccx43` because of the account dedicated-core quota, so Crabbox fell back to `cpx62`.

AWS uses flexible EC2 Spot candidate pools:

```text
standard  c7a.8xlarge, c7i.8xlarge, m7a.8xlarge, m7i.8xlarge, c7a.4xlarge
fast      c7a.16xlarge, c7i.16xlarge, m7a.16xlarge, m7i.16xlarge, c7a.12xlarge, c7a.8xlarge
large     c7a.24xlarge, c7i.24xlarge, m7a.24xlarge, m7i.24xlarge, r7a.24xlarge, c7a.16xlarge, c7a.12xlarge
beast     c7a.48xlarge, c7i.48xlarge, m7a.48xlarge, m7i.48xlarge, r7a.48xlarge, c7a.32xlarge, c7i.32xlarge, m7a.32xlarge, c7a.24xlarge, c7a.16xlarge
```

Set `CRABBOX_SERVER_TYPE` or pass `--type` to use another EC2 type such as `c8a.24xlarge`.

## Cloudflare Deployment

Worker source lives in `worker/`.

Local checks:

```sh
npm ci --prefix worker
npm run format:check --prefix worker
npm run lint --prefix worker
npm run check --prefix worker
npm test --prefix worker
npm run build --prefix worker
```

Deploy:

```sh
export CLOUDFLARE_API_TOKEN="$CRABBOX_CLOUDFLARE_API_TOKEN"
export CLOUDFLARE_ACCOUNT_ID="$CRABBOX_CLOUDFLARE_ACCOUNT_ID"
npx wrangler deploy --config worker/wrangler.jsonc
```

Required Worker secrets:

```text
HETZNER_TOKEN
AWS_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY
CRABBOX_SHARED_TOKEN
```

The Worker is deployed at:

```text
https://crabbox-coordinator.steipete.workers.dev
```

The Cloudflare route `crabbox.clawd.bot/*` is also attached and currently protected by Cloudflare Access.

## OpenClaw Verification

Verified from `/Users/steipete/Projects/openclaw` on a Cloudflare-created fallback `cpx62` runner:

```sh
CI=1 /usr/bin/time -p /Users/steipete/Projects/crabbox/bin/crabbox run --id cbx_f60f47cbc879 -- pnpm test:changed:max
```

Result:

- 61 Vitest shards completed successfully.
- End-to-end warm wall time was 93.66 seconds through the Cloudflare coordinator path.
- The timing includes rsync scan, remote Git hydration, command execution, and output streaming.

For the fastest dedicated-core verification, raise the Hetzner dedicated-core quota and re-run on `ccx63`.

## Configuration

Config file:

```yaml
broker:
  url: https://crabbox-coordinator.steipete.workers.dev
  provider: aws
  token: ...
class: beast
capacity:
  market: spot
  strategy: most-available
  fallback: on-demand-after-120s
aws:
  region: eu-west-1
  rootGB: 400
lease:
  idleTimeout: 30m
  ttl: 90m
ssh:
  key: ~/.ssh/id_ed25519
  user: crabbox
  port: "2222"
```

Environment variables remain supported for automation and direct-provider debug:

```text
HCLOUD_TOKEN or HETZNER_TOKEN     Hetzner Cloud API token
AWS_PROFILE/AWS_*                AWS SDK credentials for direct --provider aws fallback
CRABBOX_PROFILE                  default default
CRABBOX_PROVIDER                 default hetzner
CRABBOX_CONFIG                   optional config file override
CRABBOX_COORDINATOR              optional broker URL override
CRABBOX_COORDINATOR_TOKEN        optional broker bearer token override
CRABBOX_DEFAULT_CLASS            default beast
CRABBOX_IDLE_TIMEOUT             default 30m
CRABBOX_TTL                      default 90m
CRABBOX_SERVER_TYPE              provider-specific override
CRABBOX_HETZNER_LOCATION         default fsn1
CRABBOX_HETZNER_IMAGE            default ubuntu-24.04
CRABBOX_HETZNER_SSH_KEY          default crabbox-steipete
CRABBOX_AWS_REGION               default eu-west-1
CRABBOX_AWS_AMI                  optional Ubuntu AMI override
CRABBOX_AWS_SECURITY_GROUP_ID    optional security group override
CRABBOX_AWS_SUBNET_ID            optional subnet override
CRABBOX_AWS_INSTANCE_PROFILE     optional IAM instance profile name
CRABBOX_AWS_ROOT_GB              default 400
CRABBOX_CAPACITY_MARKET          spot or on-demand
CRABBOX_CAPACITY_STRATEGY        most-available, price-capacity-optimized, capacity-optimized, or sequential
CRABBOX_CAPACITY_FALLBACK        default on-demand-after-120s
CRABBOX_CAPACITY_REGIONS         comma-separated AWS region candidates for Spot placement score
CRABBOX_CAPACITY_AVAILABILITY_ZONES comma-separated AWS availability zone candidates
CRABBOX_SSH_KEY                  default ~/.ssh/id_ed25519
CRABBOX_SSH_USER                 default crabbox
CRABBOX_SSH_PORT                 default 2222
CRABBOX_WORK_ROOT                default /work/crabbox
CRABBOX_SYNC_CHECKSUM            opt into checksum rsync
CRABBOX_SYNC_DELETE              opt into/out of rsync --delete
CRABBOX_SYNC_GIT_SEED            opt into/out of remote Git seeding
CRABBOX_SYNC_FINGERPRINT         opt into/out of no-op sync skipping
CRABBOX_SYNC_BASE_REF            default base ref to hydrate
CRABBOX_SYNC_TIMEOUT             default 15m
CRABBOX_SYNC_WARN_FILES/BYTES    large-sync warning thresholds
CRABBOX_SYNC_FAIL_FILES/BYTES    large-sync failure thresholds
CRABBOX_SYNC_ALLOW_LARGE         bypass large-sync failure thresholds
CRABBOX_RESULTS_JUNIT            comma-separated remote JUnit XML paths
CRABBOX_CACHE_PNPM/NPM/DOCKER/GIT opt into/out of cache command kinds
CRABBOX_CACHE_MAX_GB             cache policy size hint
CRABBOX_CACHE_PURGE_ON_RELEASE   purge cache on release policy hint
CRABBOX_ENV_ALLOW                comma-separated env allowlist
CRABBOX_OWNER                    bearer-auth usage owner override
CRABBOX_ORG                      bearer-auth usage org
CRABBOX_COST_RATES_JSON          explicit hourly USD cost-rate overrides
CRABBOX_EUR_TO_USD               Hetzner EUR-to-USD conversion, default 1.08
CRABBOX_MAX_ACTIVE_LEASES        fleet active-lease limit
CRABBOX_MAX_ACTIVE_LEASES_PER_OWNER
CRABBOX_MAX_ACTIVE_LEASES_PER_ORG
CRABBOX_MAX_MONTHLY_USD          fleet reserved monthly spend limit
CRABBOX_MAX_MONTHLY_USD_PER_OWNER
CRABBOX_MAX_MONTHLY_USD_PER_ORG
```

Forwarded environment is intentionally narrow and project-configured:

- `NODE_OPTIONS`
- `CI`

Do not pass secret values as command-line arguments. Keep provider tokens outside the repository.

## Development

Build from source:

```sh
go build -o bin/crabbox ./cmd/crabbox
```

Run the local gate:

```sh
gofmt -w $(git ls-files '*.go')
go vet ./...
go test -race ./...
scripts/check-go-coverage.sh 85.0
go build -trimpath -o bin/crabbox ./cmd/crabbox
goreleaser release --snapshot --clean --skip=publish
npm ci --prefix worker
npm run format:check --prefix worker
npm run lint --prefix worker
npm run check --prefix worker
npm test --prefix worker
npm run build --prefix worker
```

CI runs the same checks on pushes and pull requests.

## Releases

Tagged pushes matching `v*` publish Go CLI archives through GoReleaser. Manual reruns can use the `release` workflow with a tag input.

Homebrew users install the published formula from `https://github.com/openclaw/homebrew-tap` with one command:

```sh
brew install openclaw/tap/crabbox
```

The release workflow needs `HOMEBREW_TAP_GITHUB_TOKEN` with write access to that tap repository.

## Docs

- [docs/architecture.md](docs/architecture.md)
- [docs/orchestrator.md](docs/orchestrator.md)
- [docs/cli.md](docs/cli.md)
- [docs/commands/README.md](docs/commands/README.md)
- [docs/infrastructure.md](docs/infrastructure.md)
- [docs/source-map.md](docs/source-map.md)
- [docs/mvp-plan.md](docs/mvp-plan.md)
- [docs/security.md](docs/security.md)
- [CHANGELOG.md](CHANGELOG.md)

## License

Crabbox is released under the MIT License. See [LICENSE](LICENSE).
