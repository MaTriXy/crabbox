# doctor

`crabbox doctor` checks local prerequisites and broker/provider access.

```sh
crabbox doctor
crabbox doctor --provider aws
```

It checks local tools, per-lease key generation support, coordinator health when configured, and direct-provider API access otherwise. If `CRABBOX_SSH_KEY` is explicitly set, it also validates that private key and matching `.pub` file.

Flags:

```text
--provider hetzner|aws
```
