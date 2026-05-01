# Coordinator

Read when:

- changing brokered lease behavior;
- debugging coordinator auth, health, pool, status, or usage;
- deciding whether behavior belongs in the CLI or Worker.

The coordinator is the Cloudflare Worker plus Fleet Durable Object. Normal Crabbox operation goes through this broker; direct provider mode is for debugging and escape hatches.

Responsibilities:

- authenticate broker requests with the shared token and Cloudflare Access context when present;
- serialize fleet state in one Durable Object;
- create, heartbeat, release, expire, and look up leases;
- own provider credentials;
- create and delete provider resources;
- list the pool;
- enforce cost and active-lease guardrails;
- expose usage statistics.

API surface:

```text
GET  /v1/health
GET  /v1/pool
POST /v1/leases
GET  /v1/leases
GET  /v1/leases/{id}
POST /v1/leases/{id}/heartbeat
POST /v1/leases/{id}/release
GET  /v1/usage
```

The CLI owns local config, per-lease SSH keys, SSH readiness, sync, command execution, output streaming, and local fallback handling.

Related docs:

- [Orchestrator](../orchestrator.md)
- [Architecture](../architecture.md)
- [CLI](../cli.md)
- [usage command](../commands/usage.md)
