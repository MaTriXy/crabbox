# Sprites

Read when:

- choosing `provider: sprites`;
- debugging Sprites token, CLI, SSH proxy, or bootstrap behavior;
- changing Sprites lease creation, status, sync, or cleanup.

`provider: sprites` creates a Sprites Linux microVM and adapts it into a normal
Crabbox SSH lease. Sprites owns the microVM lifecycle and `sprite proxy`.
Crabbox owns local config, slugs, repo claims, SSH keys, rsync sync, command
execution, timing summaries, and normalized list/status output.

## Auth

Prefer environment variables or user config. Do not commit Sprites tokens in
repo config.

```sh
export SPRITES_TOKEN=...
```

Fallback names are also accepted:

```sh
export CRABBOX_SPRITES_TOKEN=...
export SPRITE_TOKEN=...
export SETUP_SPRITE_TOKEN=...
```

Install and authenticate the Sprites CLI first. Crabbox uses the Sprites API for
create/delete and calls the local CLI only for `sprite --version`,
`sprite exec`, and `sprite proxy`.

## Config

```yaml
provider: sprites
target: linux
sprites:
  apiUrl: https://api.sprites.dev
  workRoot: /home/sprite/crabbox
```

Equivalent one-off flags:

```sh
crabbox warmup --provider sprites
crabbox run --provider sprites --sprites-work-root /home/sprite/crabbox -- pnpm test
crabbox ssh --provider sprites --id <slug>
crabbox status --provider sprites --id <slug>
crabbox stop --provider sprites <slug>
```

## Behavior

- `warmup` creates a `crabbox-...` sprite and local Crabbox claim.
- Crabbox bootstraps OpenSSH server, Git, rsync, tar, and python3 inside the
  sprite, writes the per-lease public key to
  `/home/sprite/.ssh/authorized_keys`, and starts `sshd`.
- `run` creates or reuses a sprite, syncs the current Git manifest over SSH,
  and runs the command through Crabbox's standard SSH executor.
- `ssh` prints a command that uses `sprite proxy -s %h -W 22` as the SSH
  `ProxyCommand`.
- `status`, `list`, and `stop` operate on Sprites resources that Crabbox can
  map to local claims or provider labels.
- `stop` deletes the sprite and removes the local claim after provider cleanup
  succeeds.

## Boundaries

- Linux only.
- No Crabbox coordinator; Sprites API auth is local/provider-native.
- No VNC, desktop, browser, or code-server.
- Actions hydration can run because Sprites exposes a normal Linux SSH target.
- `--class` and `--type` are not used for Sprites.

## Troubleshooting

- `missing Sprites token`: set `CRABBOX_SPRITES_TOKEN`, `SPRITES_TOKEN`,
  `SPRITE_TOKEN`, or `SETUP_SPRITE_TOKEN`.
- `missing sprite CLI`: install the authenticated Sprites CLI and ensure
  `sprite` is on `PATH`.
- `sprite proxy` failures mean SSH cannot reach the microVM even if API calls
  work. Run `crabbox status --provider sprites --id <slug> --wait` to retry the
  idempotent SSH bootstrap.
- Slow first boot usually means package install inside the sprite is still
  running. Kept leases reuse the installed OpenSSH/rsync packages.
- Work roots must be dedicated absolute paths under the sprite user's home, for
  example `/home/sprite/crabbox`.

Related docs:

- [Provider: Sprites](../providers/sprites.md)
- [Providers](providers.md)
- [Provider backends](../provider-backends.md)
