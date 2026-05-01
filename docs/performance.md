# Performance

Read when:

- making remote runs faster;
- choosing machine classes;
- changing sync behavior;
- tuning Actions hydration.

Crabbox performance comes from avoiding repeated setup, keeping the sync small, choosing available capacity, and reusing project-defined hydration when it matters.

## Warm Leases

Use `warmup` for repeated agent loops:

```sh
bin/crabbox warmup --class beast --idle-timeout 90m
bin/crabbox run --id cbx_... -- pnpm test:changed:max
```

Warm leases avoid waiting for a fresh VM and preserve package caches outside the synced source tree. Use `crabbox stop cbx_...` when the loop is done.

## Sync Fingerprints

The CLI records a local/remote fingerprint after sync. If nothing changed, hot runs skip the expensive rsync pass.

Good habits:

- keep generated artifacts and dependency folders out of the synced tree;
- tune repo-local excludes in `.crabbox.yaml`;
- avoid broad local deletes unless they are intentional;
- use `inspect` when diagnosing stale remote state.

## Git Hydration

Crabbox seeds remote Git when possible, then overlays the dirty local checkout with rsync. It also hydrates configured base-ref history so changed-file commands can compare against the expected base.

This matters for commands such as:

```sh
pnpm test:changed:max
pnpm check:changed
git diff --name-only origin/main...
```

## Package And Tool Caches

Runner bootstrap prepares shared package cache locations for Node, pnpm, Docker, Git, and build tools. These caches are best-effort speedups and must not be treated as source of truth.

Use explicit cache commands on kept leases:

```sh
bin/crabbox cache stats --id cbx_...
bin/crabbox cache warm --id cbx_... -- pnpm install --frozen-lockfile
bin/crabbox cache purge --id cbx_... --kind pnpm --force
```

For repeatable setup that needs repository secrets, use Actions hydration:

```sh
bin/crabbox actions hydrate --id cbx_...
bin/crabbox run --id cbx_... -- pnpm test:changed:max
```

The workflow owns dependency installation, cache/service setup, and secret-backed environment hydration. Crabbox attaches later commands to the hydrated workspace.

## Machine Classes

Use the smallest class that keeps the target command CPU-bound without creating queue or quota failures.

Typical choices:

- `standard`: cheap smoke checks and small repos.
- `fast`: general maintainer testing.
- `large`: broad test shards or heavy builds.
- `beast`: high-core changed-test runs.

Hetzner dedicated classes can hit account quota. AWS Spot classes can hit regional capacity. For AWS, `CRABBOX_CAPACITY_STRATEGY=most-available` and multiple `CRABBOX_CAPACITY_REGIONS` give the coordinator more room to find capacity.

## Measure The Loop

Use wall-clock timing around the whole command, not just the remote test process:

```sh
/usr/bin/time -p bin/crabbox run --id cbx_... -- pnpm test:changed:max
```

The useful number includes lease wait, SSH readiness, sync, Git hydration, command execution, and release. For warm leases, sync fingerprints and package caches should make repeated runs much faster than cold runs.
