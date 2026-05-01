# MVP Plan

This is the historical MVP design record. For current implementation ownership, use [Source Map](source-map.md); for user-facing behavior, use [How Crabbox Works](how-it-works.md), [CLI](cli.md), and [Features](features/README.md).

## Goal

Build Crabbox as a Go CLI plus Cloudflare coordinator that lets trusted OpenClaw maintainers run local worktrees on shared remote machines with a fast local-agent loop:

1. Ask for a machine class.
2. Get an idle warm machine or provision a new Hetzner machine.
3. Sync the local dirty tree.
4. Run a command remotely with streamed output.
5. Release or clean up the machine automatically.

The MVP should optimize for a useful maintainer workflow, not generalized cloud scheduling.

## Product Shape

Primary one-shot command:

```sh
crabbox run --profile openclaw-check -- pnpm check:changed
```

Primary agent loop:

```sh
crabbox warmup --profile openclaw-check
crabbox run --id cbx_abcdef123456 -- pnpm check:changed
crabbox stop cbx_abcdef123456
```

Expected user experience:

- Human-readable progress by default.
- Machine-readable `--json` for scripts.
- No central project secrets store in MVP.
- Local env allowlist only.
- Shared pool for trusted maintainers.
- Warm machines for fast repeated checks.
- `warmup` is first-class, because hydrated boxes shorten the agent feedback loop.
- One-shot `run --profile ...` is convenience sugar over acquire, sync, run, and release.
- TTL cleanup for abandoned leases.
- Explicit `stop`/`release` for manual cleanup.

## Product Boundary

Crabbox MVP is an OpenClaw-specific remote testbox loop, not a drop-in replacement for every hosted runner behavior.

Crabbox MVP runs commands over SSH on owned cloud capacity. Actions-backed hydration can run project setup inside a real GitHub Actions job, but direct SSH runs must be explicit about:

- secrets being forwarded from local env only by allowlist;
- no GitHub Actions OIDC or repository secret access in MVP;
- no untrusted multi-tenant execution;
- weaker isolation until per-lease users or disposable machines are implemented;
- caching being local warm-machine state rather than a central cache service.

## Repositories

Current implementation lives in one repo:

- `openclaw/crabbox`: Go CLI, Worker coordinator, docs, release/deploy scripts, and the OpenClaw plugin package.

A separate desired-state fleet repo can still exist later, but it is not part of the current 0.1.0 implementation. Live lease state belongs in Cloudflare Durable Objects.

## MVP Components

Build in this order:

1. Repo scaffold
   - Go module.
   - `cmd/crabbox`.
   - `worker/` or `services/coordinator/` for Cloudflare Worker code.
   - `docs/`, `configs/`, `scripts/`.
   - CI with build, format, and focused tests.

2. Config loading
   - Flags override env.
   - Env overrides repo-local `crabbox.yaml`.
   - Repo-local config overrides user config.
   - Defaults fill anything unset.

3. Coordinator API
   - Cloudflare Worker validates shared bearer-token auth for non-health routes.
   - Cloudflare Access can protect custom routes in front of the Worker.
   - Durable Object owns lease state and atomic machine selection.
   - Worker calls Hetzner and AWS APIs for create/delete/status.
   - Worker exposes JSON API under `/v1`.

4. Lease lifecycle
   - `POST /v1/leases` acquires or provisions.
   - `POST /v1/leases/{id-or-slug}/heartbeat` keeps lease alive.
   - `POST /v1/leases/{id-or-slug}/release` releases or deletes.
   - Durable Object alarm reaps expired leases.
   - Machines have states: `idle`, `leased`, `draining`, `provisioning`, `failed`.

5. SSH runner
   - MVP transport: public SSH to Hetzner, key-only, locked-down `crabbox` user.
   - CLI receives machine address and SSH username from the coordinator.
   - CLI owns rsync, command execution, streaming output, and exit code propagation.
   - Prefer per-lease generated SSH keys over a long-lived shared maintainer key.
   - Later transport: Cloudflare Tunnel/Access SSH or SSH CA.

6. Sync
   - Use `rsync` for MVP.
   - Preserve local dirty tree, including uncommitted changes.
   - Exclude heavy local folders by profile: `node_modules`, `.turbo`, `.git/lfs`, caches.
   - Sync to `/work/crabbox/<lease-id>/<repo-name>`.
   - Remote workdir must remain a valid Git checkout when commands depend on changed-file detection.
   - Preferred sync model: warm-clone/fetch the repo at the requested base ref, then rsync the local working tree overlay with deletes.
   - Record sync metadata for debugging.

7. Hetzner backend
   - Create machines from configured image.
   - Attach configured SSH key.
   - Apply labels: `crabbox=true`, `profile=...`, `lease=...`, `slug=...`, `owner=...`, `last_touched_at=...`, `idle_timeout_secs=...`, `ttl_secs=...`, `expires_at=...`.
   - Support warm static pool and ephemeral overflow.
   - Implement cleanup for stale ephemeral machines.

8. OpenClaw profile
   - `openclaw-check` profile.
   - Linux x64 with Crabbox bootstrap plumbing only; Docker, Node 24, pnpm, and other project runtimes come from repo-owned setup or Actions hydration.
   - Default TTL: 90 minutes.
   - Default machine class configurable, likely `ccx33` first.
   - Env allowlist: `OPENCLAW_*`, `NODE_OPTIONS`, common model/provider keys only when explicitly configured locally.
   - Persistent warm-machine caches for pnpm and Docker are allowed, but must be separated from synced source state and documented as best-effort speedups.

9. Access/auth
   - Primary org: GitHub `openclaw`.
   - Cloudflare Access org: `openclaw-crabbox.cloudflareaccess.com`.
   - Cloudflare OTP remains available for early fallback.
   - GitHub OAuth app exists under the `openclaw` org as `Crabbox Access`.
   - GitHub IdP exists in Cloudflare Access as `GitHub OpenClaw`.
   - Fallback Access app exists for `crabbox.clawd.bot`.

10. Usability pass
    - `crabbox doctor`.
    - Helpful errors for missing `rsync`, SSH key, config, Access token, or provider token.
    - `--json` for every state-inspecting command.
    - Shell completions.

## Definition Of Done

MVP is done when this works from a local OpenClaw checkout:

```sh
crabbox login
crabbox run --profile openclaw-check -- pnpm check:changed
```

And proves:

- A lease is created.
- A Hetzner machine is selected or provisioned.
- Local files sync.
- Remote command output streams.
- The local exit code matches the remote command exit code.
- Lease is released on success/failure.
- Expired leases are cleaned by idle timeout and TTL cap.
- Machine pool state is visible through `crabbox pool`.

## Non-Goals For MVP

- No Kubernetes.
- No central secret storage.
- No full autoscaling scheduler.
- No multi-tenant untrusted execution.
- No Windows/macOS workers.
- No hosted-runner adapter in the first implementation path.
- No attempt to perfectly hide SSH; make it reliable first.

## Known Current Infra Facts

- Direct CLI execution is implemented and verified. It can create/reuse a Hetzner server, bootstrap it, sync a local checkout with rsync, hydrate shallow Git history enough for changed-test detection, run commands over SSH, stream output, and release/delete leases.
- The Cloudflare coordinator and Durable Object lease store are implemented and deployed. The CLI uses them when `CRABBOX_COORDINATOR` is set, and falls back to direct Hetzner otherwise.
- Intended primary domain: `crabbox.openclaw.ai`.
- Current Cloudflare-manageable fallback domain: `crabbox.clawd.bot`.
- `openclaw.ai` must be visible as a Cloudflare zone before `crabbox.openclaw.ai/*` can be attached as a Worker route. Current public DNS is on Namecheap nameservers.
- Cloudflare account ID and Crabbox Cloudflare token are available in local and MacBook Pro `~/.profile`.
- The current Crabbox Cloudflare token is `crabbox-deploy`, scoped to `Steipete@gmail.com's Account` and the `clawd.bot` zone.
- The current Crabbox Cloudflare token verifies Workers scripts, Access apps, Access IdPs, Access keys, DNS records, and zone Worker routes.
- Cloudflare Access is enabled.
- Current Access IdPs are OTP and GitHub.
- GitHub OAuth app `Crabbox Access` exists under the `openclaw` org for Cloudflare Access.
- Crabbox browser login uses a GitHub OAuth callback at `/v1/auth/github/callback` and stores OAuth client values as Worker secrets.
- Cloudflare Access GitHub IdP `GitHub OpenClaw` exists.
- Cloudflare Access app `Crabbox Coordinator` exists for `crabbox.clawd.bot`.
- Worker `crabbox-coordinator` is deployed at `https://crabbox-coordinator.steipete.workers.dev` and routed from `crabbox.clawd.bot/*`. The canonical target is `https://crabbox.openclaw.ai` once the Cloudflare zone is delegated.
- Coordinator auth supports GitHub browser-login user tokens plus shared-token operator automation. Shared-token auth uses `CRABBOX_COORDINATOR_TOKEN` locally and `CRABBOX_SHARED_TOKEN` in the Worker.
- Hetzner token is available in local and Mac Studio `~/.profile`.
- The Hetzner account currently hits a dedicated-core quota/resource limit for `ccx63`, `ccx53`, and `ccx43`. The `beast` class falls back to `cpx62` until quota is raised.
- Public SSH on port 22 was not usable from the tested network path; cloud-init opens SSH on port 2222 and the CLI uses that by default.
- OpenClaw verification through the Cloudflare coordinator on the fallback `cpx62` runner passed `CI=1 pnpm test:changed:max`, completing 61 Vitest shards in 93.66 seconds end-to-end for a warm run, including rsync scan and remote Git hydration.
- GitHub org slug is `openclaw`.
- `wrangler` and `hcloud` are not assumed to be globally installed; use `npx wrangler` and direct Hetzner API or document install steps.

## Next Implementation Milestones

1. Raise Hetzner dedicated-core quota so `beast` can use `ccx63` instead of falling back to `cpx62`.
2. Add GitHub org/team allowlisting for browser-login user tokens.
3. Delegate `openclaw.ai` to Cloudflare or provide a token that can create/manage that zone, then attach `crabbox.openclaw.ai/*`.
4. Add Cloudflare Access service-token support for non-browser CLI use on fallback routes.
5. Add one-shot `run --profile` cleanup semantics coverage in integration tests.
6. Add coordinator drain controls beyond release/delete.
7. Re-run OpenClaw `pnpm test:changed:max` on `ccx63` and compare against the current Crabbox baseline.
