# Changelog

## Unreleased

### Added

- Native OpenClaw plugin package exposing Crabbox run, warmup, status, list, and stop tools.

## 0.1.0 - 2026-05-01

Initial Crabbox release.

### Added

- Go CLI for leasing remote Linux test boxes, syncing dirty worktrees, running commands over SSH, and cleaning up.
- Cloudflare Worker coordinator with Durable Object lease state.
- Hetzner and AWS EC2 Spot provisioning with class-based fallback.
- Brokered lease flow with heartbeats, idle expiry, TTL caps, release, pool listing, and status lookup.
- Direct-provider fallback mode for Hetzner and AWS debugging.
- Commands: `init`, `doctor`, `warmup`, `run`, `status`, `list`, `ssh`, `inspect`, `stop`, `cleanup`, and `config`.
- Login and identity commands: `login`, `logout`, and `whoami`.
- Trusted operator admin command for listing, releasing, and deleting coordinator leases.
- Run history and retained run-log tails via `history` and `logs`.
- JUnit test-result summaries via `run --junit` and `results`.
- Cache controls via `cache stats`, `cache warm`, and `cache purge`.
- Usage command for estimated cost and runtime reporting by user, org, or fleet.
- Sync inspection via `sync-plan`, showing local sync candidates, largest files, and largest directories without leasing a box.
- GitHub Actions bridge with `actions register`, `actions dispatch`, and `actions hydrate` for running project-owned workflow setup on leased boxes.
- Hydrated workspace detection so `crabbox run --id <lease>` syncs local dirty work into the workflow's `$GITHUB_WORKSPACE`.
- Git-based sync manifests so Crabbox transfers tracked files plus nonignored untracked files instead of the full local tree.
- Sync preflight estimates, large-sync guardrails, timeout control, and quiet-rsync heartbeats.
- Orchestrator cost guardrails for active leases and monthly reserved spend.
- Provider-backed pricing from AWS Spot price history and Hetzner server-type prices, with static fallback rates.
- Compatibility aliases: `release`, `pool list`, and `machine cleanup`.
- Per-lease SSH key generation under the user config directory.
- Per-lease cloud SSH key/key-pair cleanup when machines are deleted.
- Stable canonical lease IDs plus friendly crustacean-style lease slugs, such as `blue-lobster`.
- Slug-aware lease lookup for active leases while keeping `cbx_...` IDs canonical for APIs, labels, paths, SSH keys, and scripts.
- Provider-visible server names and runner labels that include readable slugs while retaining canonical lease labels.
- Idle auto-free for kept leases, defaulting to `30m`, with `--ttl` retained as the maximum wall-clock lifetime cap.
- Local Crabbox claim state so reused leases stay associated with the repo that acquired them unless reclaimed explicitly.
- `--reclaim` support for moving a local lease claim between repositories intentionally.
- Checksum rsync with delete semantics and a remote sanity guard for mass tracked deletions.
- Shallow Git hydration for changed-test workflows.
- Repo onboarding via `crabbox init`, generating `.crabbox.yaml`, `.github/workflows/crabbox.yml`, and `.agents/skills/crabbox/SKILL.md`.
- Command docs under `docs/commands/` plus architecture, orchestrator, CLI, infrastructure, MVP, and security docs.
- GoReleaser archive configuration for macOS, Linux, and Windows.
- Homebrew tap publishing through `https://github.com/openclaw/homebrew-tap` for `brew install openclaw/tap/crabbox`.

### Changed

- Top-level help is now workflow-first, with common flows, grouped commands, config pointers, environment variables, and aliases.
- `--idle-timeout` now controls inactivity expiry; `--ttl` remains the maximum lease lifetime.
- Repo config is YAML-only; pre-release JSON compatibility was removed before shipping.
- Default sync excludes now keep `.git`, common dependency folders, and local build caches out of runner transfers.
- `actions hydrate` inspects workflow-dispatch inputs when possible and skips undeclared optional fields.
- `doctor` accepts per-lease SSH keys as the default posture and validates explicit `CRABBOX_SSH_KEY` only when set.
- Coordinator requests bound dial/TLS timeouts and fall back to local `curl` on transport failures.
- Local per-lease SSH keys move with coordinator-renamed lease IDs.
- Cache stats and purge honor repo cache-kind toggles.
- Stored test-result summaries are bounded before run history persistence.
- `run`, `warmup`, and `actions hydrate` print concise completion timing summaries for faster live-test comparisons.
- Human output now includes slugs alongside canonical lease IDs when available, and JSON output exposes slug and idle-expiry fields additively.
- Plain `status` remains read-only, while active operations such as `run`, `ssh`, cache commands, and Actions hydration touch leases.

### Fixed

- Config-writing commands honor `CRABBOX_CONFIG`, so isolated login/logout tests do not touch the normal user config.
- Boolean flags for `logs` and admin lease actions work after positional IDs, such as `crabbox logs run_... --json`.
- `actions hydrate` retries without optional `crabbox_job` when an older workflow ref rejects the input.
- `cache warm` uses the hydrated GitHub Actions workspace and env handoff when a lease was prepared by `actions hydrate`.
- Remote Git worktrees store sync metadata under `.git/crabbox` so repository status stays clean without using the worktree `.crabbox/` directory.
- Touch-only heartbeats no longer overwrite an existing lease's idle timeout unless the caller explicitly updates it.
- Direct-provider slugs are collision-checked against active servers before provisioning.
- Direct-provider cleanup expiry is capped by the shorter of idle timeout and TTL.
