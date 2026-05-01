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
- Install with `brew install openclaw/tap/crabbox`.
- Auth is required for brokered operation:
  `printf '%s' "$CRABBOX_COORDINATOR_TOKEN" | crabbox login --url https://crabbox-coordinator.steipete.workers.dev --provider aws --token-stdin`.
- User config lives at `~/Library/Application Support/crabbox/config.yaml` on
  macOS or the platform user config dir elsewhere. It should contain:

```yaml
broker:
  url: https://crabbox-coordinator.steipete.workers.dev
  token: <token>
provider: aws
```

## Common Flow

Warm a reusable box:

```sh
crabbox warmup --idle-timeout 90m
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
crabbox logs <run_id>
crabbox results <run_id>
crabbox cache stats --id <id-or-slug>
crabbox ssh --id <id-or-slug>
crabbox usage --scope org
```

Use `--debug` on `run` when measuring sync timing.

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
