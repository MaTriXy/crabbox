# Architecture

## System Overview

Crabbox has three main parts:

- CLI: local Go binary used by maintainers and agents.
- Coordinator: Cloudflare Worker plus Durable Object state.
- Workers: Hetzner or SSH-accessible machines that run commands.

The coordinator leases machines. The CLI executes work. Machines do not need to call back to the coordinator in the MVP.

```text
developer laptop
  crabbox CLI
    |
    | HTTPS JSON API, Cloudflare Access
    v
Cloudflare Worker
  Durable Object lease state
    |
    | Hetzner API or AWS EC2 Spot API
    v
cloud machines

developer laptop
  |
  | SSH + rsync
  v
leased machine
```

## Lease Flow

1. CLI loads config and authenticates to Cloudflare Access.
2. CLI creates a per-lease SSH key.
3. CLI sends `POST /v1/leases` with lease ID, slug, profile, TTL, idle timeout, desired machine class, and SSH public key.
4. Coordinator validates identity and policy.
5. Durable Object chooses a provider from config and creates a Hetzner server or AWS EC2 Spot instance.
6. Coordinator returns lease ID, slug, machine address, SSH user, workdir, and expiry.
7. CLI waits for `crabbox-ready`.
8. CLI seeds remote Git when possible, compares sync fingerprints, and syncs changed files with `rsync --delete`.
9. CLI runs sync sanity and configured base-ref hydration.
10. CLI runs the command over SSH and streams stdout/stderr.
11. CLI heartbeats while the command runs; heartbeats touch `lastTouchedAt` and recompute idle expiry up to the TTL cap.
12. CLI releases the lease when done.
13. Durable Object alarm cleans up stale leases and expired machines.

## Coordinator API

MVP endpoints:

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

Admin endpoints can be gated by GitHub team or explicit allowlist once GitHub IdP is active.

## Durable Object State

Use one fleet Durable Object for MVP. It owns all atomic scheduling decisions.

Core tables:

```sql
machines(id, provider, provider_id, profile, class, state, address, ssh_user, labels_json, lease_id, created_at, updated_at, last_seen_at)
leases(id, owner, org, profile, machine_id, state, command, repo, ttl_seconds, estimated_hourly_usd, max_estimated_usd, expires_at, created_at, updated_at, released_at)
events(id, lease_id, machine_id, type, actor, message, payload_json, created_at)
```

State transitions:

```text
machine: provisioning -> idle -> leased -> idle
machine: provisioning -> failed
machine: leased -> draining -> idle|deleted
lease: pending -> active -> released
lease: pending|active -> expired
lease: active -> failed
```

## Backends

Owned backends:

- `hetzner-static`: pre-created warm machines.
- `hetzner-ephemeral`: created per lease or overflow.
- `aws-spot`: one-time EC2 Spot instances for burst capacity.
- `ssh-static`: manually managed machines reachable by SSH.

Brokered backends, later:

- `github-actions`: register or dispatch real Actions-backed runner work when workflow parity is required.
- `external-runner`: adapter boundary for other hosted runner systems if needed.

The MVP implements `hetzner-ephemeral` and `aws-spot`, and leaves interfaces ready for `hetzner-static`.

## Machine Bootstrap

Bootstrap should produce machines with:

- `crabbox` user.
- SSH key-only auth.
- Docker.
- Git.
- Node 24.
- pnpm.
- build-essential.
- rsync.
- writable `/work/crabbox`.
- cleanup service or boot-time cleanup script.

Prefer snapshots/images once bootstrap is proven. Cloud-init is acceptable for first pass.

## Config Sources

Config precedence:

```text
flags > env > repo-local crabbox.yaml/.crabbox.yaml > user config > defaults
```

User config is YAML and can define:

- coordinator URL.
- coordinator bearer token.
- profiles.
- machine classes.
- backend defaults.
- sync excludes.
- env allowlists.
- capacity market/strategy/fallback.
- Actions workflow/job/ref hints.
- trusted projects.
- sync behavior such as checksum mode, Git seeding, and fingerprint skipping.

It must not store:

- live leases.
- SSH private keys.
- provider secrets.

Per-lease SSH private keys live under the user config directory, outside repo config. Provider secrets live in the broker environment, such as Cloudflare Worker secrets for AWS and Hetzner.

## Failure Model

Assume:

- CLI can crash.
- SSH can disconnect.
- Machines can fail boot.
- Hetzner API calls can race or partially complete.
- Cloudflare Worker can retry requests.

Therefore:

- Lease creation must be idempotent where practical.
- TTL cleanup must be authoritative.
- Provider resources need labels for orphan cleanup.
- Release should be safe to call multiple times.
- Machine delete should tolerate already-deleted resources.
