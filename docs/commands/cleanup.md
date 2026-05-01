# cleanup

`crabbox cleanup` sweeps direct-provider leftovers.

```sh
crabbox cleanup --dry-run
crabbox cleanup
```

Cleanup refuses to run when a coordinator is configured. Brokered cleanup belongs to the Durable Object alarm.

Direct cleanup skips kept machines and active states. It deletes clearly expired inactive machines and stale active-state machines after an extra safety window.

Flags:

```text
--provider hetzner|aws
--dry-run
```

`crabbox machine cleanup` remains as a compatibility alias.
