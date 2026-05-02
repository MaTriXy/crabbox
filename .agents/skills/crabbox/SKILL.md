---
name: crabbox
description: Use Crabbox for remote Linux validation, warmed reusable boxes, GitHub Actions hydration, sync timing, logs, results, caches, and lease cleanup.
---

# Crabbox

Use Crabbox when a project needs remote Linux proof, larger cloud capacity,
warm reusable runner state, GitHub Actions hydration, or fast sync from a dirty
local checkout.

## Before Running

- Run from the repository root. Crabbox sync mirrors the current checkout.
- Prefer local targeted tests for tight edit loops.
- Check repo-local `crabbox.yaml` or `.crabbox.yaml` before adding flags.
- Sanity-check the selected binary before remote work:
  `command -v crabbox && crabbox --version && crabbox --help | sed -n '1,80p'`.
- Install with `brew install openclaw/tap/crabbox`.
- Auth is required for brokered operation. Normal users run `crabbox login`.
- Trusted operator automation can store the shared token with:
  `printf '%s' "$CRABBOX_COORDINATOR_TOKEN" | crabbox login --url https://crabbox.openclaw.ai --provider aws --token-stdin`.
- User config lives at `~/Library/Application Support/crabbox/config.yaml` on
  macOS or the platform user config dir elsewhere. It should contain:

```yaml
broker:
  url: https://crabbox.openclaw.ai
  token: <token>
provider: aws
```

## Common Flow

Warm a reusable box:

```sh
crabbox warmup --idle-timeout 90m
crabbox warmup --provider aws --class beast --market on-demand --idle-timeout 90m
```

Hydrate it through a repository GitHub Actions workflow when CI-like setup,
services, or secret-backed preparation are needed:

```sh
crabbox actions hydrate --id <cbx_id-or-slug>
```

Run commands:

```sh
crabbox run --id <cbx_id-or-slug> -- pnpm test:changed
crabbox run --id <cbx_id-or-slug> --shell "corepack enable && pnpm install --frozen-lockfile && pnpm test"
```

For package-manager commands on raw AWS/Hetzner boxes, hydrate first when the
repo declares an Actions workflow; bootstrap only installs Crabbox plumbing, not
project runtimes. Add `--timing-json` when comparing providers or sync phases.

Stop boxes you created before handoff:

```sh
crabbox stop <cbx_id-or-slug>
```

## Useful Commands

```sh
crabbox status --id <id-or-slug> --wait
crabbox inspect --id <id-or-slug> --json
crabbox sync-plan
crabbox history --lease <id-or-slug>
crabbox events <run_id> --json
crabbox attach <run_id>
crabbox logs <run_id>
crabbox results <run_id>
crabbox cache stats --id <id-or-slug>
crabbox ssh --id <id-or-slug>
crabbox usage --scope org
CRABBOX_LIVE=1 CRABBOX_LIVE_REPO=/path/to/openclaw scripts/live-smoke.sh
```

## Run Inspection Workflow

Use the CLI for durable run inspection; do not expect extra OpenClaw plugin
tools for this surface.

Find recent runs:

```sh
crabbox history --limit 20
crabbox history --lease <id-or-slug> --limit 20
```

Follow an active run:

```sh
crabbox attach <run_id>
crabbox attach <run_id> --after <seq>
```

Page through recorded events:

```sh
crabbox events <run_id> --after <seq> --limit 100
crabbox events <run_id> --json
```

Inspect completed output and structured test summaries:

```sh
crabbox logs <run_id>
crabbox results <run_id>
```

Use `--debug` on `run` when measuring sync timing.
Use `--timing-json` on `warmup`, `actions hydrate`, and `run` when a stable
machine-readable timing record is needed.
Use `--market spot|on-demand` on AWS `warmup` or one-shot `run` when account
quota or capacity testing needs a temporary market override.

## Run Handles

Coordinator-backed `crabbox run` prints `recording run run_...` before leasing
starts. Keep that run ID in status updates. Use `crabbox events run_...` for
ordered lifecycle/output events, `crabbox attach run_...` to follow an active
run, and `crabbox logs run_...` or `crabbox results run_...` after completion.

Output events are a capped preview, not unlimited logs. Use `logs` for the
retained command output tail when debugging noisy runs.

## Hydration Boundary

Repository setup belongs in the repository hydration workflow. That workflow
owns checkout, runtime setup, dependencies, services, secret-backed preparation,
the ready marker, and keepalive.

Crabbox owns runner registration, workflow dispatch, SSH sync, command
execution, logs/results, local lease claims, and idle cleanup. Do not add
project-specific setup to the Crabbox binary.

## Cleanup

Brokered leases have coordinator-owned idle expiry and local lease claims, so
projects should not maintain their own lease ledger. Default idle timeout is 30
minutes unless config or flags set a different value. Still stop boxes you
created when done.

When `crabbox list` prints `orphan=no-active-lease`, treat it as an operator
review hint: verify the provider machine is not referenced by an active
coordinator lease before deleting anything, especially if `keep=true` is set.
