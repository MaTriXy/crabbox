# Auth And Admin

Read when:

- changing broker login or identity;
- changing trusted operator controls;
- debugging who owns a lease or run.

Crabbox currently supports bearer-token broker auth. `crabbox login` stores the broker URL, provider, and token in the user config, then verifies the token with `GET /v1/whoami`. It is not yet a GitHub browser OAuth flow.

Identity sent to the coordinator:

```text
Cloudflare Access email, when present
X-Crabbox-Owner from CRABBOX_OWNER, Git email env, or git config user.email
X-Crabbox-Org from CRABBOX_ORG
CRABBOX_DEFAULT_ORG fallback in the Worker
```

Commands:

```sh
crabbox login --url <url> --token-stdin
crabbox whoami
crabbox logout
```

Trusted operator controls:

```sh
crabbox admin leases --state active
crabbox admin release blue-lobster
crabbox admin delete cbx_... --force
```

Admin commands use the same coordinator token as normal broker calls. Do not distribute the shared token to untrusted users. A future access-control pass should split operator and user tokens before Crabbox is opened beyond trusted maintainers.

Related docs:

- [login](../commands/login.md)
- [whoami](../commands/whoami.md)
- [admin](../commands/admin.md)
- [Security](../security.md)
