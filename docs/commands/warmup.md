# warmup

`crabbox warmup` provisions or leases a remote box and waits until SSH and the toolchain are ready.

```sh
crabbox warmup --class beast --idle-timeout 90m
```

The command returns a `cbx_...` lease ID. Reuse that ID for subsequent `run`, `status`, `ssh`, `inspect`, and `stop` commands.

Flags:

```text
--provider hetzner|aws
--profile <name>
--class <name>
--type <provider-type>
--ttl <duration>
--idle-timeout <duration>
--keep
```

`--idle-timeout` is the preferred name for agent workflows. It maps to the same lease expiry as `--ttl`.

New leases use per-lease SSH keys under the user config directory:

```text
~/.config/crabbox/testboxes/<lease-id>/id_ed25519
```
