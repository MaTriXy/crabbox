# login

`crabbox login` opens GitHub in the browser, waits for the coordinator callback, stores the returned broker token in the user config, and verifies identity with `GET /v1/whoami`.

```sh
crabbox login
```

If the browser cannot open automatically, print the URL and paste it manually:

```sh
crabbox login --no-browser
```

Trusted operator automation can still write the shared coordinator token over stdin:

```sh
printf '%s' "$CRABBOX_COORDINATOR_TOKEN" | crabbox login \
  --url https://crabbox.openclaw.ai \
  --provider aws \
  --token-stdin
```

Secrets are read from stdin so they do not land in shell history.

Flags:

```text
--url <url>                 broker URL
--provider hetzner|aws      default provider to store with the broker
--no-browser                print the GitHub login URL instead of opening it
--token-stdin               read broker token from stdin for operator automation
--json                      print JSON
```

The default broker URL is `https://crabbox.openclaw.ai`; pass `--url` for another coordinator. GitHub browser login issues a user-scoped Crabbox bearer token. `--token-stdin` stores the shared operator token and should stay limited to trusted maintainers.

Related docs:

- [whoami](whoami.md)
- [logout](logout.md)
- [Broker auth and routing](../features/broker-auth-routing.md)
