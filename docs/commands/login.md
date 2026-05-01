# login

`crabbox login` stores broker credentials in the user config and verifies coordinator identity. It is currently token-based, not a GitHub browser OAuth flow.

```sh
printf '%s' "$CRABBOX_COORDINATOR_TOKEN" | crabbox login \
  --url https://crabbox-coordinator.steipete.workers.dev \
  --provider aws \
  --token-stdin
```

Secrets are read from stdin so they do not land in shell history.

Flags:

```text
--url <url>                 broker URL
--provider hetzner|aws      default provider to store with the broker
--token-stdin               read broker token from stdin
--json                      print JSON
```

`login` calls `GET /v1/whoami` after writing config. If verification fails, inspect the stored config with `crabbox config show` and retry with the correct token.

The coordinator may still derive identity from Cloudflare Access or Git email headers, but the CLI does not yet open a browser or mint a GitHub-scoped user token.

Related docs:

- [whoami](whoami.md)
- [logout](logout.md)
- [Broker auth and routing](../features/broker-auth-routing.md)
