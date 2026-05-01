# MVP Plan

## Goal

Build Crabbox as a Go CLI plus Cloudflare coordinator that lets trusted OpenClaw maintainers run local worktrees on shared remote machines with the same core feel as Blacksmith Testboxes:

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
crabbox run --id cbx_123 -- pnpm check:changed
crabbox stop cbx_123
```

Expected user experience:

- Human-readable progress by default.
- Machine-readable `--json` for scripts.
- No central project secrets store in MVP.
- Local env allowlist only.
- Shared pool for trusted maintainers.
- Warm machines for fast repeated checks.
- `warmup` is first-class, because the Blacksmith Testbox value comes from hydrating a box before the agent needs test feedback.
- One-shot `run --profile ...` is convenience sugar over acquire, sync, run, and release.
- TTL cleanup for abandoned leases.
- Explicit `stop`/`release` for manual cleanup.

## Product Boundary

Crabbox MVP is an OpenClaw-specific replacement for the useful local-agent loop of Blacksmith Testboxes, not a drop-in replacement for all Blacksmith runner behavior.

Blacksmith Testboxes run commands inside a real GitHub Actions job, including GitHub Actions secrets, OIDC tokens, service containers, and their runner image. Crabbox MVP instead runs commands over SSH on owned Hetzner capacity. This is acceptable for trusted OpenClaw maintainers, but it means the MVP must be explicit about:

- secrets being forwarded from local env only by allowlist;
- no GitHub Actions OIDC or repository secret access in MVP;
- no untrusted multi-tenant execution;
- weaker isolation until per-lease users or disposable machines are implemented;
- caching being local warm-machine state rather than Blacksmith colocated cache or sticky disks.

## Repositories

Use two repos:

- `openclaw/crabbox`: Go CLI, Worker coordinator, docs, deploy scripts.
- `openclaw/crabbox-fleet`: desired fleet config only.

The fleet repo is not a lock database. It stores profiles, machine classes, default TTLs, sync excludes, and backend declarations. Live lease state belongs in Cloudflare Durable Objects.

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
   - User config overrides shared fleet config.
   - Shared fleet config can be fetched from GitHub raw content or local checkout.

3. Coordinator API
   - Cloudflare Worker validates Cloudflare Access JWT.
   - Durable Object owns lease state and atomic machine selection.
   - Worker calls Hetzner API for create/delete/status.
   - Worker exposes JSON API under `/v1`.

4. Lease lifecycle
   - `POST /v1/leases` acquires or provisions.
   - `POST /v1/leases/{id}/heartbeat` keeps lease alive.
   - `POST /v1/leases/{id}/release` releases or deletes.
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
   - Apply labels: `crabbox=true`, `profile=...`, `lease=...`, `owner=...`.
   - Support warm static pool and ephemeral overflow.
   - Implement cleanup for stale ephemeral machines.

8. OpenClaw profile
   - `openclaw-check` profile.
   - Linux x64, Docker, Node 24, pnpm, Git.
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
- Expired leases are cleaned by TTL.
- Machine pool state is visible through `crabbox pool`.

## Non-Goals For MVP

- No Kubernetes.
- No central secret storage.
- No full autoscaling scheduler.
- No multi-tenant untrusted execution.
- No Windows/macOS workers.
- No Blacksmith backend in the first implementation path.
- No attempt to perfectly hide SSH; make it reliable first.

## Known Current Infra Facts

- Direct CLI execution is implemented and verified. It can create/reuse a Hetzner server, bootstrap it, sync a local checkout with rsync, hydrate shallow Git history enough for changed-test detection, run commands over SSH, stream output, and release/delete leases.
- The Cloudflare coordinator and Durable Object lease store are implemented and deployed. The CLI uses them when `CRABBOX_COORDINATOR` is set, and falls back to direct Hetzner otherwise.
- Intended primary domain: `crabbox.openclaw.ai`.
- Current Cloudflare-manageable fallback domain: `crabbox.clawd.bot`.
- `openclaw.ai` is currently not visible as a Cloudflare zone in the available account; DNS is on Namecheap nameservers.
- Cloudflare account ID and Crabbox Cloudflare token are available in local and MacBook Pro `~/.profile`.
- The current Crabbox Cloudflare token is `crabbox-deploy`, scoped to `Steipete@gmail.com's Account` and the `clawd.bot` zone.
- The current Crabbox Cloudflare token verifies Workers scripts, Access apps, Access IdPs, Access keys, DNS records, and zone Worker routes.
- Cloudflare Access is enabled.
- Current Access IdPs are OTP and GitHub.
- GitHub OAuth app `Crabbox Access` exists under the `openclaw` org.
- GitHub OAuth client ID and secret are present in local and MacBook Pro `~/.profile`.
- Cloudflare Access GitHub IdP `GitHub OpenClaw` exists.
- Cloudflare Access app `Crabbox Coordinator` exists for `crabbox.clawd.bot`.
- Worker `crabbox-coordinator` is deployed at `https://crabbox-coordinator.steipete.workers.dev` and routed from `crabbox.clawd.bot/*`.
- Coordinator bearer auth uses `CRABBOX_COORDINATOR_TOKEN` locally and `CRABBOX_SHARED_TOKEN` in the Worker.
- Hetzner token is available in local and Mac Studio `~/.profile`.
- The Hetzner account currently hits a dedicated-core quota/resource limit for `ccx63`, `ccx53`, and `ccx43`. The `beast` class falls back to `cpx62` until quota is raised.
- Public SSH on port 22 was not usable from the tested network path; cloud-init opens SSH on port 2222 and the CLI uses that by default.
- OpenClaw verification through the Cloudflare coordinator on the fallback `cpx62` runner passed `CI=1 pnpm test:changed:max`, completing 61 Vitest shards in 93.66 seconds end-to-end for a warm run, including rsync scan and remote Git hydration.
- GitHub org slug is `openclaw`.
- `wrangler` and `hcloud` are not assumed to be globally installed; use `npx wrangler` and direct Hetzner API or document install steps.

## Next Implementation Milestones

1. Raise Hetzner dedicated-core quota so `beast` can use `ccx63` instead of falling back to `cpx62`.
2. Add `crabbox login` and Cloudflare Access token handling.
3. Add Cloudflare Access service-token support for non-browser CLI use on `crabbox.clawd.bot`.
4. Add heartbeat support for long-running commands.
5. Add one-shot `run --profile` cleanup semantics coverage in integration tests.
6. Add coordinator admin cleanup/drain endpoints.
7. Re-run OpenClaw `pnpm test:changed:max` on `ccx63` and compare against Blacksmith Testboxes.
