# warmup

`crabbox warmup` provisions or leases a remote box and waits until SSH and the toolchain are ready.

```sh
crabbox warmup --class beast --idle-timeout 90m
crabbox warmup --actions-runner --idle-timeout 90m
```

The command returns a `cbx_...` lease ID. Reuse that ID for subsequent `run`, `status`, `ssh`, `inspect`, and `stop` commands.

On success, `warmup` prints a concise total duration line.

Flags:

```text
--provider hetzner|aws
--profile <name>
--class <name>
--type <provider-type>
--ttl <duration>
--idle-timeout <duration>
--keep
--actions-runner
```

`--idle-timeout` is the preferred name for agent workflows. It maps to the same lease expiry as `--ttl`.

`--actions-runner` immediately registers the warm box as an ephemeral self-hosted GitHub Actions runner for the current repository. Most projects should prefer `crabbox actions hydrate --id <lease>` after warmup because it also dispatches the workflow and waits for the ready marker.

New leases use per-lease SSH keys under the user config directory:

```text
~/.config/crabbox/testboxes/<lease-id>/id_ed25519
```
