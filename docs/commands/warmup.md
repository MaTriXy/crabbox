# warmup

`crabbox warmup` provisions or leases a remote box and waits until SSH and Crabbox bootstrap plumbing are ready.

```sh
crabbox warmup --class beast
crabbox warmup --actions-runner
crabbox warmup --provider blacksmith-testbox --blacksmith-workflow .github/workflows/ci-check-testbox.yml --blacksmith-job test
```

The command returns a stable `cbx_...` lease ID and a friendly slug. Reuse either for subsequent `run`, `status`, `ssh`, `inspect`, and `stop` commands; scripts should keep using the canonical ID.

With `--provider blacksmith-testbox`, the canonical ID is the Blacksmith `tbx_...` ID returned by `blacksmith testbox warmup`; Crabbox still assigns and stores a local slug for reuse.

On success, `warmup` prints a concise total duration line.

Flags:

```text
--provider hetzner|aws|blacksmith-testbox
--profile <name>
--class <name>
--type <provider-type>
--ttl <duration>
--idle-timeout <duration>
--keep
--actions-runner
--reclaim
--blacksmith-org <org>
--blacksmith-workflow <file|name|id>
--blacksmith-job <job>
--blacksmith-ref <ref>
```

`--idle-timeout` releases the lease after no touch for that duration, default `30m`. `--ttl` remains the maximum wall-clock lifetime, default `90m`.
Warmup records a local claim tying the lease to the current repo; `--reclaim` overwrites an existing local claim for that lease.

`--actions-runner` immediately registers the warm box as an ephemeral self-hosted GitHub Actions runner for the current repository. Most projects should prefer `crabbox actions hydrate --id <lease-id-or-slug>` after warmup because it also dispatches the workflow and waits for the ready marker.

`--actions-runner` is not supported with `blacksmith-testbox` because Blacksmith owns Testbox workflow hydration.

New leases use per-lease SSH keys under the user config directory:

```text
~/.config/crabbox/testboxes/<lease-id>/id_ed25519
```
