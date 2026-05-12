# Static SSH Provider

Read when:

- choosing `provider: ssh`, `provider: static`, or `provider: static-ssh`;
- reusing an existing Linux, macOS, or Windows host;
- changing `internal/providers/ssh` or static-host sync behavior.

Static SSH is the provider for machines Crabbox does not create. The backend
resolves a configured SSH target and then core owns sync, command execution,
results, tunnels, and status rendering.

## When To Use

Use Static SSH when:

- the machine already exists and should not be provisioned by Crabbox;
- you need a local Mac Studio, LAN host, VM, or persistent Windows box;
- cloud provider cleanup and cost accounting do not apply.

Use AWS, Azure, or Hetzner when you want Crabbox to create and delete the machine.

## Commands

```sh
crabbox run --provider ssh --static-host mac-studio.local -- pnpm test
crabbox ssh --provider ssh --id mac-studio.local
crabbox run --provider static-ssh --target windows --static-host win-dev.local -- pwsh -NoProfile -Command '$PSVersionTable'
```

`warmup` for Static SSH does not provision a new machine. It validates and
returns the configured target as a lease-like object for common workflows.

## Linux Or macOS Config

```yaml
provider: ssh
target: macos
static:
  host: mac-studio.local
  user: steipete
  port: "22"
  workRoot: /Users/steipete/crabbox
```

Linux uses the same POSIX contract:

```yaml
provider: ssh
target: linux
static:
  host: buildbox.local
  user: crabbox
  port: "22"
  workRoot: /work/crabbox
```

## Windows Config

Native Windows mode uses PowerShell over OpenSSH and archive sync:

```yaml
provider: ssh
target: windows
windows:
  mode: normal
static:
  host: win-dev.local
  user: Peter
  port: "22"
  workRoot: C:\crabbox
```

WSL2 mode keeps the POSIX contract inside WSL:

```yaml
provider: ssh
target: windows
windows:
  mode: wsl2
static:
  host: win-dev.local
  user: Peter
  port: "22"
  workRoot: /home/peter/crabbox
```

## Host Requirements

POSIX hosts need:

- SSH access for the configured user;
- `git`, `rsync`, `tar`, `sh`, and a writable `static.workRoot`;
- optional desktop/browser/code tooling if those flags are requested.

Windows native hosts need:

- OpenSSH server;
- PowerShell;
- tar support for archive sync;
- optional TightVNC/browser tooling for desktop flows.

WSL2 hosts need:

- WSL installed and reachable through `wsl.exe`;
- Linux tools inside the default WSL distribution;
- `static.workRoot` as a WSL path.

## Capabilities

- SSH: yes.
- Crabbox sync: yes.
- Desktop/browser/code: host-dependent.
- Tailscale: use the host's existing tailnet address or MagicDNS name.
- Actions hydration: Linux SSH hosts only.
- Coordinator: no.

## Gotchas

- Crabbox does not clean up static hosts. `stop` removes local claims only.
- Static hosts can drift. Run `crabbox doctor` and a small `crabbox run` before
  long jobs.
- `target` and `windows.mode` must match the real host. Crabbox cannot infer
  whether a Windows host should run native PowerShell or WSL2 commands.

Related docs:

- [Provider overview](README.md)
- [Sync](../features/sync.md)
- [SSH keys](../features/ssh-keys.md)
- [Provider backends](../provider-backends.md)
