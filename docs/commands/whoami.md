# whoami

`crabbox whoami` verifies broker auth and prints the identity the coordinator sees.

```sh
crabbox whoami
crabbox whoami --json
```

Human output:

```text
user=steipete@gmail.com org=openclaw auth=github broker=https://crabbox.openclaw.ai
```

Identity comes from Cloudflare Access email when present, then signed GitHub login tokens, then bearer-token headers. In shared bearer-token mode, the CLI sends `X-Crabbox-Owner` from `CRABBOX_OWNER`, Git email env, or `git config user.email`, and `X-Crabbox-Org` from `CRABBOX_ORG`.

Related docs:

- [login](login.md)
- [Broker auth and routing](../features/broker-auth-routing.md)
