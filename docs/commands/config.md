# config

`crabbox config` manages user config.

```sh
crabbox config path
crabbox config show
crabbox config show --json
printf '%s' "$TOKEN" | crabbox config set-broker --url https://crabbox-coordinator.steipete.workers.dev --provider aws --token-stdin
```

Subcommands:

```text
path
show [--json]
set-broker --url <url> --token-stdin [--provider hetzner|aws]
```

User config lives under the OS user config directory. Repo-local `.crabbox.json` can override user defaults for a checkout.
