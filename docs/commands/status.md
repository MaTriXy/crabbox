# status

`crabbox status` prints the current state for a lease.

```sh
crabbox status --id blue-lobster
crabbox status --id blue-lobster --wait --wait-timeout 10m
crabbox status --id blue-lobster --json
```

`--id` accepts the canonical `cbx_...` ID or active slug. Plain status is read-only; `--wait` touches the lease while waiting.

Flags:

```text
--id <lease-id-or-slug>
--provider hetzner|aws
--wait
--wait-timeout <duration>
--json
```
