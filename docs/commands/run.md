# run

`crabbox run` syncs the current dirty checkout to a box, runs a command, streams output, and returns the remote exit code.

```sh
crabbox run --id cbx_123 -- pnpm test:changed:max
crabbox run --class beast --idle-timeout 90m -- pnpm check
```

If `--id` is omitted, Crabbox creates a fresh non-kept lease and releases it when the command exits.

Sync uses `rsync --delete --checksum` over SSH. This mirrors local deletions and avoids stale remote files. Use `--debug` to print sync timing and itemized rsync output.

After sync, Crabbox runs a remote sanity check. If the remote checkout reports at least 200 tracked deletions, Crabbox fails before running tests unless local `OPENCLAW_TESTBOX_ALLOW_MASS_DELETIONS=1` is set.

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
--debug
```
