# inspect

`crabbox inspect` prints detailed lease and provider metadata.

```sh
crabbox inspect --id cbx_123
crabbox inspect --id cbx_123 --json
```

Use this for debugging coordinator state, provider labels, expiry, and SSH target details.

Flags:

```text
--id <lease-id>
--provider hetzner|aws
--json
```
