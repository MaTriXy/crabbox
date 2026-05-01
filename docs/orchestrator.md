# Orchestrator

Crabbox has one orchestrator: the Cloudflare Worker plus Fleet Durable Object. The CLI can still talk directly to Hetzner or AWS for debugging, but normal operation should go through the coordinator.

## Responsibilities

The orchestrator owns:

- lease IDs and lease state;
- provider credentials;
- server creation and deletion;
- idle expiry and heartbeat renewal;
- pool listing;
- status lookup.

The CLI owns:

- local config;
- per-lease SSH key creation;
- SSH readiness waits;
- rsync of the dirty working tree;
- remote command execution;
- output streaming.

## Lease States

Current user-facing states:

```text
provisioning
active
ready
running
released
expired
failed
```

The Worker stores coordinator leases as `active`, `released`, `expired`, or `failed`. Provider labels add finer local runner state such as `leased`, `ready`, and `running`.

## Heartbeats And Idle Timeout

`crabbox warmup --idle-timeout 90m` and `crabbox run --idle-timeout 90m` map to lease TTL. The CLI sends coordinator heartbeats while a lease is in use. Each heartbeat extends `expiresAt` by the original TTL.

Direct-provider mode does not have a central heartbeat. It labels machines with `created_at`, `expires_at`, and `state`; `crabbox cleanup` uses those labels conservatively.

## Cleanup

Brokered cleanup is owned by the Durable Object alarm. `crabbox cleanup` refuses to run when a coordinator is configured, because sweeping provider resources behind the coordinator can delete live leases.

Direct cleanup only deletes machines that are clearly safe:

- `keep=true` is skipped;
- active states are skipped;
- expired inactive machines can be deleted;
- stale active states older than expiry plus 12 hours can be deleted.

## Blacksmith Parity Boundary

Blacksmith Testboxes run inside a real GitHub Actions job with Actions secrets, OIDC, and service containers. Crabbox currently runs commands over SSH on owned cloud capacity. That is useful for maintainer verification and agent loops, but it is not full Actions parity.

The current bridge is `crabbox init`: generate repo-local workflow and agent instructions so warmup can hydrate the same dependencies the real CI uses. A future backend can register ephemeral self-hosted runners or dispatch Actions-backed testboxes for full secrets/OIDC parity.
