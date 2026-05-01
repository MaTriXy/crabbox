# Changelog

## 0.1.0 - 2026-05-01

Initial Crabbox release.

### Added

- Go CLI for leasing remote Linux test boxes, syncing dirty worktrees, running commands over SSH, and cleaning up.
- Cloudflare Worker coordinator with Durable Object lease state.
- Hetzner and AWS EC2 Spot provisioning with class-based fallback.
- Brokered lease flow with heartbeats, TTL expiry, release, pool listing, and status lookup.
- Direct-provider fallback mode for Hetzner and AWS debugging.
- Commands: `init`, `doctor`, `warmup`, `run`, `status`, `list`, `ssh`, `inspect`, `stop`, `cleanup`, and `config`.
- Compatibility aliases: `release`, `pool list`, and `machine cleanup`.
- Per-lease SSH key generation under the user config directory.
- Per-lease cloud SSH key/key-pair cleanup when machines are deleted.
- Checksum rsync with delete semantics and a remote sanity guard for mass tracked deletions.
- Shallow Git hydration for changed-test workflows.
- Repo onboarding via `crabbox init`, generating `.crabbox.json`, `.github/workflows/crabbox.yml`, and `.agents/skills/crabbox/SKILL.md`.
- Command docs under `docs/commands/` plus architecture, orchestrator, CLI, infrastructure, MVP, and security docs.
- GoReleaser archive configuration for macOS, Linux, and Windows.

### Changed

- Top-level help is now workflow-first, with common flows, grouped commands, config pointers, environment variables, and aliases.
- `--idle-timeout` is documented as the preferred agent-facing name for lease TTL.
- `doctor` accepts per-lease SSH keys as the default posture and validates explicit `CRABBOX_SSH_KEY` only when set.
