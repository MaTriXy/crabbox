# Sprites Provider

Read when:

- choosing `provider: sprites`;
- configuring Sprites tokens, API URL, or work root;
- changing `internal/providers/sprites`.

Sprites is an SSH lease provider for Linux microVMs. Crabbox creates a Sprites
sprite through the Sprites API, bootstraps OpenSSH inside it, then uses
`sprite proxy` as the SSH `ProxyCommand`. Crabbox owns slugs, local repo claims,
per-lease SSH keys, rsync, command execution, timing summaries, and normalized
list/status rendering.

## When To Use

Use Sprites when you want a short-lived Linux microVM with normal Crabbox
SSH/rsync behavior. Use AWS, Azure, or Hetzner when you need brokered fleet
accounting, VNC/desktop/code, provider firewall control, or cloud images.

## Commands

```sh
crabbox warmup --provider sprites
crabbox run --provider sprites -- pnpm test
crabbox ssh --provider sprites --id blue-lobster
crabbox status --provider sprites --id blue-lobster
crabbox stop --provider sprites blue-lobster
```

## Live Smoke

Use a live smoke when changing Sprites lifecycle, SSH bootstrap, proxy command,
or cleanup behavior. Keep the token in the environment or user config; do not
pass it as a command-line argument.

```sh
export SPRITES_TOKEN=...
go build -trimpath -o bin/crabbox ./cmd/crabbox

bin/crabbox warmup --provider sprites --timing-json
lease=<slug-or-cbx_id-from-warmup-output>

bin/crabbox status --provider sprites --id "$lease" --wait
bin/crabbox ssh --provider sprites --id "$lease"
bin/crabbox run --provider sprites --id "$lease" --shell 'echo crabbox-sprites-ok'
bin/crabbox list --provider sprites
bin/crabbox stop --provider sprites "$lease"
```

Expected results:

- `warmup` creates a `crabbox-...` sprite, prints `provider=sprites`, a
  Crabbox lease ID, slug, and sprite name.
- `status --wait` reports a running Linux lease.
- `ssh` prints an SSH command that includes
  `ProxyCommand=sprite proxy -s %h -W 22`.
- The run prints `crabbox-sprites-ok`.
- `stop` deletes the sprite and removes the local lease claim and key.

## Auth

```sh
export SPRITES_TOKEN=...
```

`CRABBOX_SPRITES_TOKEN` is also accepted and wins over `SPRITES_TOKEN`.
`SPRITE_TOKEN` and `SETUP_SPRITE_TOKEN` are accepted for compatibility with the
Sprites installer.

The authenticated `sprite` CLI must be on `PATH`; Crabbox validates it with
`sprite --version` before creating a lease.

## Configuration

```yaml
provider: sprites
target: linux
sprites:
  apiUrl: https://api.sprites.dev
  workRoot: /home/sprite/crabbox
```

Flags: `--sprites-api-url`, `--sprites-work-root`.

Environment variables:

```text
CRABBOX_SPRITES_TOKEN
SPRITES_TOKEN
SPRITE_TOKEN
SETUP_SPRITE_TOKEN
CRABBOX_SPRITES_API_URL
SPRITES_API_URL
CRABBOX_SPRITES_WORK_ROOT
```

## Lifecycle

1. Create a Sprites sprite named `crabbox-<slug>`.
2. Generate a per-lease Crabbox SSH key.
3. Install OpenSSH server, Git, rsync, tar, and python3 inside the sprite, add
   the public key for user `sprite`, and start `sshd`.
4. Return a normal Crabbox SSH target with
   `ProxyCommand=sprite proxy -s %h -W 22`.
5. Crabbox syncs and runs commands over SSH.
6. Delete the sprite on release unless the lease is kept.

## Capabilities

- SSH: yes, through `sprite proxy`.
- Crabbox sync: yes, standard SSH/rsync path.
- Desktop/browser/code: no.
- Actions hydration: yes, because Sprites returns a Linux SSH target.
- Coordinator: no.

## Gotchas

- IDs can be Crabbox lease IDs, local slugs, `spr_<sprite-name>` IDs, or raw
  sprite names.
- Existing raw sprites can be reclaimed only with `--reclaim`.
- `--class` and `--type` are rejected because Sprites owns VM sizing.
- Work roots must be dedicated absolute paths. Broad roots such as `/`,
  `/home`, `/tmp`, and `/home/sprite` are rejected before sync.
- SSH depends on the local `sprite` CLI. If `sprite proxy` cannot connect,
  `status --wait`, `run`, and `ssh` fail even if the API can see the sprite.
- `list` shows Crabbox-owned sprites whose names or labels start with
  `crabbox-`.

Related docs:

- [Feature: Sprites](../features/sprites.md)
- [Provider backends](../provider-backends.md)
