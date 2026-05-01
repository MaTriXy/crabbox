# run

`crabbox run` syncs the current dirty checkout to a box, runs a command, streams output, and returns the remote exit code.

```sh
crabbox run --id cbx_123 -- pnpm test:changed:max
crabbox run --class beast --idle-timeout 90m -- pnpm check
crabbox run --id cbx_123 --shell 'pnpm install --frozen-lockfile && pnpm test'
crabbox run --id cbx_123 --junit junit.xml -- go test ./...
```

If `--id` is omitted, Crabbox creates a fresh non-kept lease and releases it when the command exits.

When the lease has been hydrated by `crabbox actions hydrate`, `run` reads the remote marker under `$HOME/.crabbox/actions`, syncs into the workflow's `$GITHUB_WORKSPACE`, and sources the non-secret env file written by the workflow. That preserves the setup the workflow performed: checkout path, installed dependencies, service containers, caches, runner temp/toolcache paths, and any project-specific preparation. GitHub secrets and OIDC request tokens remain workflow-step scoped unless the project explicitly persists its own short-lived credentials.

Sync uses `rsync --delete` over SSH by default. Crabbox records a local/remote sync fingerprint and skips rsync when the tracked commit plus dirty files have not changed. Use `--checksum` when you need a paranoid checksum scan, and `--debug` to print sync timing and itemized rsync output.

Before the first rsync into a Git checkout, Crabbox tries to seed the remote worktree from the local `origin` remote so the first sync is a dirty-tree overlay instead of a full source upload. Project-specific excludes, env forwarding, and base ref belong in `crabbox.yaml` or `.crabbox.yaml`.

After sync, Crabbox runs a remote sanity check. If the remote checkout reports at least 200 tracked deletions, Crabbox fails before running tests unless local `CRABBOX_ALLOW_MASS_DELETIONS=1` is set.

When a coordinator is configured, Crabbox records each remote command as a run history item. `crabbox history` lists those records and `crabbox logs <run-id>` prints the retained remote output tail. Log retention is intentionally bounded so a noisy command cannot fill Durable Object storage.

Add `--junit <path>` or configure `results.junit` to attach JUnit XML summaries to the run record. `crabbox results <run-id>` then prints failed tests without reading the raw log tail.

Flags:

```text
--id <lease-id>
--provider hetzner|aws
--profile <name>
--class <name>
--type <provider-type>
--ttl <duration>
--idle-timeout <duration>
--keep
--no-sync
--sync-only
--shell
--checksum
--debug
--junit <comma-separated remote XML paths>
```
