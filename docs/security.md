# Security

## Trust Model

MVP is for trusted OpenClaw maintainers, not arbitrary untrusted users.

Assumptions:

- Users can run arbitrary commands on leased machines.
- Machines may see forwarded local env values.
- Users are trusted not to attack other users intentionally.
- Bugs and crashes still happen, so cleanup must be defensive.

## Authentication

Cloudflare Access protects the coordinator.

MVP:

- One-time PIN Access remains available for early fallback.
- GitHub Access IdP is configured for the `openclaw` org.
- Coordinator validates `Cf-Access-Jwt-Assertion`.
- Coordinator maps Access identity to lease owner.
- Workers.dev automation currently uses a shared bearer token. `crabbox login` stores and verifies that token; browser-based Cloudflare Access/GitHub OAuth and split user/admin tokens are still future hardening.
- Missing shared-token config fails closed for non-health coordinator routes.

Target:

- Keep GitHub org membership as the normal access path.
- Optional team allowlist for admin commands.

## Authorization

Roles:

```text
user: acquire, heartbeat, release own leases, list own leases
maintainer: shared warm pool access
admin: drain machines, cleanup, view all leases, deploy
```

Until GitHub teams are wired, admin identity can be an explicit allowlist in Worker config.

## Secrets

No central project secret store in MVP.

Rules:

- Secrets stay local.
- CLI forwards env only by allowlist.
- Users can opt in additional env names with repo-local `env.allow` config.
- Never accept secret values as command-line flag values.
- Never log env values.
- Redact known secret-looking strings in diagnostics.
- `CRABBOX_SHARED_TOKEN` is stored as a Worker secret; local clients use `CRABBOX_COORDINATOR_TOKEN`.

Project allowlist example:

```json
{
  "env": {
    "allow": ["CI", "NODE_OPTIONS", "PROJECT_*"]
  }
}
```

## SSH

MVP SSH posture:

- Public SSH allowed only for worker machines.
- Key-only authentication.
- Dedicated `crabbox` user.
- No password login.
- No root login.
- SSH listens on port 2222 in the verified direct-CLI path because port 22 was not reachable during Hetzner testing.
- The CLI generates per-lease SSH keys under the user config directory for new leases.
- Matching cloud SSH keys/key pairs are removed when Crabbox deletes the machine.
- Work happens under `/work/crabbox`.
- Machines are disposable or cleanable.

MVP hardening before first shared use:

- Keep long-lived maintainer keys out of machine images.
- Restrict Hetzner firewalls to known callers when practical.
- Redact command diagnostics before printing.
- Treat profiles that forward secrets as higher risk; prefer ephemeral machines for those profiles.

Later hardening:

- Cloudflare Tunnel or Access SSH.
- SSH CA with short-lived certs.
- Per-lease Unix users.
- Per-lease workdir ownership and cleanup.

## Cleanup

Cleanup is security-sensitive.

Required:

- Lease TTL.
- Heartbeat deadline.
- Explicit release.
- Durable Object alarm cleanup.
- Provider label sweep for clearly expired, inactive orphan machines.
- Boot-time cleanup of stale `/work/crabbox/*` dirs.

Direct-CLI cleanup currently uses provider labels and skips active states. When a coordinator is configured, provider-side cleanup is disabled because the Durable Object TTL alarm owns brokered cleanup.

Release must be idempotent. Delete must tolerate already-deleted provider resources.

## Data Retention

Store only operational metadata:

- lease ID.
- owner identity.
- machine ID.
- profile.
- timestamps.
- state transitions.
- command string, unless disabled.

Do not store:

- stdout/stderr logs in the coordinator for MVP.
- env values.
- file contents.
- SSH keys.

## Audit Trail

Durable Object events should record:

```text
lease.created
machine.provisioned
lease.heartbeat
lease.extended
lease.released
lease.expired
machine.drained
machine.deleted
provider.error
```

The audit trail is for debugging and cleanup, not compliance.
