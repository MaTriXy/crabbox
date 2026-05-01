# status

`crabbox status` prints the current state for a lease.

```sh
crabbox status --id cbx_123
crabbox status --id cbx_123 --wait --wait-timeout 10m
crabbox status --id cbx_123 --json
```

`--wait` blocks until the box is ready or the timeout expires. This is useful when a warmup returns quickly in future orchestrator-backed hydration flows.

Flags:

```text
--id <lease-id>
--provider hetzner|aws
--wait
--wait-timeout <duration>
--json
```
