# CLI

## Name

`crabbox`

One-liner: lease shared remote test boxes, sync local work, run commands, and clean up.

## Usage

```text
crabbox [global flags] <command> [args]
```

Global flags:

```text
-h, --help
--version
```

Primary output goes to stdout. Progress, diagnostics, and errors go to stderr. JSON output is stable enough for scripts.

## Commands

```text
crabbox doctor
crabbox login --url <url> --token-stdin [--provider hetzner|aws]
crabbox logout
crabbox whoami [--json]
crabbox init [--force]
crabbox config show [--json]
crabbox config path
crabbox config set-broker --url <url> --token-stdin [--provider hetzner|aws]
crabbox warmup [--provider hetzner|aws] [--profile <name>] [--idle-timeout <duration>]
crabbox run [--id <lease-id>] [--shell] [--checksum] [--debug] -- <command...>
crabbox history [--lease <lease-id>] [--owner <email>] [--org <name>] [--limit <n>] [--json]
crabbox logs <run-id> [--json]
crabbox results <run-id> [--json]
crabbox cache stats --id <lease-id> [--json]
crabbox cache purge --id <lease-id> --kind pnpm|npm|docker|git|all --force
crabbox cache warm --id <lease-id> -- <command...>
crabbox actions hydrate --id <lease-id> [--workflow <file|name|id>] [--wait-timeout <duration>]
crabbox actions register --id <lease-id> [--repo owner/name]
crabbox actions dispatch [--workflow <file|name|id>] [-f key=value]
crabbox status --id <lease-id> [--wait]
crabbox list [--json]
crabbox usage [--scope user|org|all] [--user <email>] [--org <name>] [--month YYYY-MM] [--json]
crabbox admin leases [--state active|released|expired|failed] [--owner <email>] [--org <name>] [--json]
crabbox admin release <lease-id> [--delete]
crabbox admin delete <lease-id> --force
crabbox ssh --id <lease-id>
crabbox inspect --id <lease-id> [--json]
crabbox stop <lease-id>
crabbox cleanup [--dry-run]
```

## Common Flows

One-shot run:

```sh
crabbox run --profile project-check -- pnpm check:changed
```

AWS EC2 Spot run:

```sh
crabbox run --class beast -- pnpm check:changed
```

Warm a box, then reuse it:

```sh
crabbox warmup --profile project-check --idle-timeout 90m
crabbox run --id cbx_123 -- pnpm test:changed
crabbox run --id cbx_123 --shell 'pnpm install --frozen-lockfile && pnpm test'
crabbox stop cbx_123
```

Hydrate through GitHub Actions, then run local dirty work in the hydrated workspace:

```sh
crabbox warmup --idle-timeout 90m
crabbox actions hydrate --id cbx_123
crabbox run --id cbx_123 -- pnpm test:changed
crabbox stop cbx_123
```

Inspect pool:

```sh
crabbox list
crabbox list --json
```

Inspect usage and estimated cost:

```sh
crabbox usage
crabbox usage --scope org --org openclaw
crabbox usage --scope all --json
```

Cleanup direct-provider leftovers:

```sh
crabbox cleanup --dry-run
crabbox cleanup
```

Cleanup is intentionally conservative: it skips kept machines and active states. When a coordinator is configured, brokered cleanup is owned by the Durable Object TTL alarm instead of provider-side sweeping.

Debug config:

```sh
crabbox doctor
crabbox whoami
crabbox config show
crabbox config show --json
```

Inspect recorded runs:

```sh
crabbox run --id cbx_123 --junit junit.xml -- go test ./...
crabbox history --lease cbx_123
crabbox logs run_123
crabbox results run_123
```

Inspect or warm caches on a kept box:

```sh
crabbox cache stats --id cbx_123
crabbox cache warm --id cbx_123 -- pnpm install --frozen-lockfile
crabbox cache purge --id cbx_123 --kind pnpm --force
```

Trusted operator lease controls:

```sh
crabbox admin leases --state active
crabbox admin release cbx_123
crabbox admin delete cbx_123 --force
```

## `run`

`crabbox run` is the main command.

Behavior:

1. Load config.
2. Acquire a lease unless `--id` is provided.
3. Verify SSH readiness.
4. Use the GitHub Actions workspace when the lease has a hydration marker.
5. Sync current repo, unless a matching sync fingerprint lets Crabbox skip rsync.
6. Seed remote Git from the configured origin/base ref before first sync when possible.
7. Run command over SSH.
8. Stream remote output and retain the latest log tail in coordinator history.
9. Heartbeat coordinator leases in the background.
10. Release lease unless `--keep` is set.
11. Exit with the remote command exit code.

Fresh non-kept leases retry once with a new machine when bootstrap never reaches SSH readiness. Existing leases and `--keep` runs are not retried automatically, so commands are not duplicated on a machine the user asked to keep. Runner bootstrap also retries apt, NodeSource, and corepack steps inside cloud-init before `crabbox-ready` is allowed to pass.

Flags:

```text
--id <lease-id>          reuse an existing lease
--provider <name>        hetzner or aws
--profile <name>        profile to run on
--class <name>          machine class override
--type <name>           provider server or instance type override
--ttl <duration>        lease TTL, default from profile
--idle-timeout <duration>
--no-sync               run without syncing
--sync-only             sync and exit
--keep                  keep lease after command exits
--shell                 run the command string through bash -lc
--checksum              use checksum rsync instead of size/time
--debug                 print sync timing and itemized rsync output
--junit <paths>         comma-separated remote JUnit XML paths to attach to run history
```

Secrets must not be accepted as flag values. Env forwarding is name-based only.

## Exit Codes

```text
0   success
1   generic Crabbox failure
2   invalid usage or config
3   auth failure
4   no capacity
5   provisioning failure
6   sync failure
7   SSH failure
8   lease expired
10+ remote command exit code when available
```

If the remote command exits with a code, `crabbox run` returns that code unless Crabbox itself failed first.

## Config Files

The implemented config format is YAML. The default path is:

```text
macOS: ~/.config/crabbox/config.yaml through XDG, or ~/Library/Application Support/crabbox/config.yaml
Linux: ~/.config/crabbox/config.yaml
repo:  crabbox.yaml or .crabbox.yaml
```

User config:

```yaml
broker:
  url: https://crabbox-coordinator.steipete.workers.dev
  provider: aws
  token: ...
profile: project-check
class: beast
capacity:
  market: spot
  strategy: most-available
  fallback: on-demand-after-120s
aws:
  region: eu-west-1
  rootGB: 400
ssh:
  key: ~/.ssh/id_ed25519
  user: crabbox
  port: "2222"
```

Set broker auth without putting the token in shell history:

```sh
printf '%s' "$TOKEN" | crabbox login \
  --url https://crabbox-coordinator.steipete.workers.dev \
  --provider aws \
  --token-stdin
```

`crabbox config set-broker` remains available for scripts that only want to edit config without verifying identity.

Repo-local config is YAML and should hold project-specific choices:

```yaml
profile: project-check
class: beast
actions:
  workflow: .github/workflows/crabbox.yml
  ref: main
  runnerLabels:
    - crabbox
sync:
  delete: true
  checksum: false
  gitSeed: true
  fingerprint: true
  baseRef: main
  exclude:
    - node_modules
    - .turbo
    - dist
env:
  allow:
    - CI
    - NODE_OPTIONS
    - PROJECT_*
results:
  junit:
    - junit.xml
cache:
  pnpm: true
  npm: true
  docker: true
  git: true
  maxGB: 80
  purgeOnRelease: false
```

## Environment Variables

```text
CRABBOX_COORDINATOR
CRABBOX_COORDINATOR_TOKEN
CRABBOX_PROVIDER
CRABBOX_PROFILE
CRABBOX_CONFIG
CRABBOX_SSH_KEY
CRABBOX_RESULTS_JUNIT
CRABBOX_CACHE_PNPM/NPM/DOCKER/GIT
CRABBOX_CACHE_MAX_GB
CRABBOX_CACHE_PURGE_ON_RELEASE
```

Provider/deploy variables live outside normal CLI operation:

```text
CRABBOX_CLOUDFLARE_API_TOKEN
CRABBOX_CLOUDFLARE_ACCOUNT_ID
CRABBOX_CLOUDFLARE_ZONE_ID
HCLOUD_TOKEN
AWS_PROFILE/AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY
GITHUB_TOKEN
```

## Output Rules

Human output:

```text
acquiring lease profile=project-check ttl=90m
leased cbx_abc123 machine=hz-ccx33-01 expires=2026-04-30T17:30:00Z
syncing 184 files -> /work/crabbox/cbx_abc123/openclaw
running pnpm check:changed
...
released cbx_abc123
```

JSON output:

```json
{
  "leaseId": "cbx_abc123",
  "machineId": "hz-ccx33-01",
  "state": "released",
  "exitCode": 0
}
```

No progress bars when stdout is not a TTY.
