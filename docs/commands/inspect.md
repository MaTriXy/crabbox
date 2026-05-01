# inspect

`crabbox inspect` prints detailed lease and provider metadata.

```sh
crabbox inspect --id blue-lobster
crabbox inspect --id blue-lobster --json
```

Use this for debugging coordinator state, provider labels, expiry, and SSH target details.

Flags:

```text
--id <lease-id-or-slug>
--provider hetzner|aws
--json
```
