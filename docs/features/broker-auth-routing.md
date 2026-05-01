# Broker Auth And Routing

Read when:

- changing coordinator authentication;
- changing Cloudflare routes or Access policy;
- debugging bearer-token automation or GitHub browser login.

The broker is exposed through Cloudflare Workers routes:

```text
https://crabbox.openclaw.ai
https://crabbox-coordinator.steipete.workers.dev
crabbox.clawd.bot/*
```

Normal users run `crabbox login`, which opens GitHub and stores a signed Crabbox user token. The coordinator needs a GitHub OAuth app with callback:

```text
https://crabbox.openclaw.ai/v1/auth/github/callback
```

Worker secrets:

```text
CRABBOX_GITHUB_CLIENT_ID
CRABBOX_GITHUB_CLIENT_SECRET
CRABBOX_SESSION_SECRET
```

Trusted automation can still use the shared operator bearer token configured in the CLI and Worker. The CLI sends:

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

GitHub user tokens are signed by the Worker and are not admin tokens. Admin routes require the shared operator token. The `crabbox.openclaw.ai/*` route is the canonical CLI and browser-login endpoint. The worker.dev and `crabbox.clawd.bot/*` routes are fallbacks.

Related docs:

- [Coordinator](coordinator.md)
- [Security](../security.md)
- [Infrastructure](../infrastructure.md)
- [config command](../commands/config.md)
