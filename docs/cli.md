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
crabbox init [--force]
crabbox config show [--json]
crabbox config path
crabbox config set-broker --url <url> --token-stdin [--provider hetzner|aws]
crabbox warmup [--provider hetzner|aws] [--profile <name>] [--idle-timeout <duration>]
crabbox run [--id <lease-id>] [--debug] -- <command...>
crabbox status --id <lease-id> [--wait]
crabbox list [--json]
crabbox ssh --id <lease-id>
crabbox inspect --id <lease-id> [--json]
crabbox stop <lease-id>
crabbox cleanup [--dry-run]
```

## Common Flows

One-shot run:

```sh
crabbox run --profile openclaw-check -- pnpm check:changed
```

AWS EC2 Spot run:

```sh
crabbox run --class beast -- pnpm check:changed
```

Warm a box, then reuse it:

```sh
crabbox warmup --profile openclaw-check --idle-timeout 90m
crabbox run --id cbx_123 -- pnpm test:changed
crabbox stop cbx_123
```

Inspect pool:

```sh
crabbox list
crabbox list --json
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
crabbox config show
crabbox config show --json
```

## `run`

`crabbox run` is the main command.

Behavior:

1. Load config.
2. Acquire a lease unless `--id` is provided.
3. Verify SSH readiness.
4. Sync current repo.
5. Verify SSH readiness again after sync and before command execution. The configured SSH port is preferred, with port 22 as a bootstrap fallback when the runner exposes default SSH first.
6. Run command over SSH.
7. Stream remote output.
8. Heartbeat coordinator leases in the background.
9. Release lease unless `--keep` is set.
10. Exit with the remote command exit code.

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
--debug                 print sync timing and itemized rsync output
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

The implemented config format is JSON. The default path is:

```text
macOS: ~/.config/crabbox/config.json through XDG, or ~/Library/Application Support/crabbox/config.json
Linux: ~/.config/crabbox/config.json
repo:  crabbox.json or .crabbox.json
```

User config:

```json
{
  "broker": {
    "url": "https://crabbox-coordinator.steipete.workers.dev",
    "provider": "aws",
    "token": "..."
  },
  "profile": "openclaw-check",
  "class": "beast",
  "aws": {
    "region": "eu-west-1",
    "rootGB": 400
  },
  "ssh": {
    "key": "~/.ssh/id_ed25519",
    "user": "crabbox",
    "port": "2222"
  }
}
```

Set broker auth without putting the token in shell history:

```sh
printf '%s' "$TOKEN" | crabbox config set-broker \
  --url https://crabbox-coordinator.steipete.workers.dev \
  --provider aws \
  --token-stdin
```

Future fleet config may become YAML:

```yaml
version: 1
defaults:
  profile: openclaw-check
  ttl: 90m
sync:
  exclude:
    - node_modules
    - .turbo
    - .git/lfs
env:
  allow:
    - OPENCLAW_*
    - NODE_OPTIONS
```

## Environment Variables

```text
CRABBOX_COORDINATOR
CRABBOX_COORDINATOR_TOKEN
CRABBOX_PROVIDER
CRABBOX_PROFILE
CRABBOX_CONFIG
CRABBOX_SSH_KEY
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
acquiring lease profile=openclaw-check ttl=90m
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
