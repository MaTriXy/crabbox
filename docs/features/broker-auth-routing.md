# Broker Auth And Routing

Read when:

- changing coordinator authentication;
- changing Cloudflare routes or Access policy;
- debugging bearer-token automation.

The broker is exposed through Cloudflare Workers routes:

```text
https://crabbox-coordinator.steipete.workers.dev
crabbox.clawd.bot/*
```

Normal automation uses a shared bearer token configured in the CLI and Worker. The CLI sends:

```text
Authorization: Bearer <token>
X-Crabbox-Owner: <email>
X-Crabbox-Org: <org>
```

Owner selection for bearer-token requests:

```text
CRABBOX_OWNER
GIT_AUTHOR_EMAIL
GIT_COMMITTER_EMAIL
git config user.email
```

`CRABBOX_ORG` sets the org header. When Cloudflare Access identity is present, Access email wins over the CLI-provided owner.

The `crabbox.clawd.bot/*` route is protected by Cloudflare Access. The worker.dev route is useful for automation and direct health checks when configured with bearer auth.

Related docs:

- [Coordinator](coordinator.md)
- [Security](../security.md)
- [Infrastructure](../infrastructure.md)
- [config command](../commands/config.md)
