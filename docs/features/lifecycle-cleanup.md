# Lifecycle Cleanup

Read when:

- changing release or expiry behavior;
- debugging leaked provider resources;
- changing direct-provider cleanup.

Brokered lifecycle is coordinator-owned:

```text
provisioning -> active -> released
provisioning -> failed
active -> expired
active -> failed
```

The CLI heartbeats active coordinator leases while a command runs. Release and expiry both call the provider delete path for non-kept machines. Kept machines remain available until explicitly stopped or expired by policy.

Brokered cleanup belongs to the Durable Object alarm. `crabbox cleanup` refuses to sweep provider resources when a coordinator is configured because that can race live brokered leases.

Direct-provider cleanup is conservative:

- skip `keep=true`;
- skip active states;
- delete clearly expired inactive machines;
- delete stale active machines only after expiry plus the extra safety window.

Provider resources should carry Crabbox labels/tags so orphan cleanup can identify them without touching unrelated infrastructure.

Related docs:

- [stop command](../commands/stop.md)
- [cleanup command](../commands/cleanup.md)
- [status command](../commands/status.md)
- [inspect command](../commands/inspect.md)
- [Security](../security.md)
