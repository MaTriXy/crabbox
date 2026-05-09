# Namespace Devbox Provider

Read when:

- choosing `provider: namespace-devbox`;
- configuring Namespace Devbox image, size, repository, site, or lifecycle;
- changing `internal/providers/namespace`.

Namespace Devbox is an SSH lease provider. The Namespace `devbox` CLI owns
auth, creation, SSH config, list, shutdown, and delete. Crabbox owns local
slugs, repo claims, dirty-tree sync, command execution, timing, and normalized
list/status output.

## When To Use

Use Namespace Devbox when the environment should come from a Namespace Devbox
image and you want Crabbox's normal SSH sync/run path on top. Use Blacksmith
Testbox when the provider should own sync and command execution through
`blacksmith testbox run`.

## Commands

```sh
devbox login
crabbox warmup --provider namespace-devbox --namespace-image builtin:base --namespace-size M
crabbox run --provider namespace-devbox --id blue-lobster -- pnpm test
crabbox ssh --provider namespace-devbox --id blue-lobster
crabbox status --provider namespace-devbox --id blue-lobster
crabbox stop --provider namespace-devbox blue-lobster
```

## Config

```yaml
provider: namespace-devbox
target: linux
namespace:
  image: builtin:base
  size: M
  repository: github.com/openclaw/crabbox
  site: ""
  volumeSizeGB: 100
  autoStopIdleTimeout: 30m
  workRoot: /workspaces/crabbox
  deleteOnRelease: false
```

Provider flags:

```text
--namespace-image
--namespace-size
--namespace-repository
--namespace-site
--namespace-volume-size-gb
--namespace-auto-stop-idle-timeout
--namespace-work-root
--namespace-delete-on-release
```

Environment overrides:

```text
CRABBOX_NAMESPACE_IMAGE
CRABBOX_NAMESPACE_SIZE
CRABBOX_NAMESPACE_REPOSITORY
CRABBOX_NAMESPACE_SITE
CRABBOX_NAMESPACE_VOLUME_SIZE_GB
CRABBOX_NAMESPACE_AUTO_STOP_IDLE_TIMEOUT
CRABBOX_NAMESPACE_WORK_ROOT
CRABBOX_NAMESPACE_DELETE_ON_RELEASE
```

## Lifecycle

1. Create a named Devbox with `devbox create --from <spec>`.
2. Call `devbox configure-ssh` and read the generated SSH config/key.
3. Wait for SSH plus `git`, `rsync`, and `tar`.
4. Store a local Crabbox lease claim and run through the normal SSH executor.
5. `crabbox stop` shuts the Devbox down by default; set
   `namespace.deleteOnRelease: true` to delete it instead.

## Capabilities

- SSH: yes.
- Crabbox sync: yes, normal rsync over SSH.
- Desktop/browser/code: no current Crabbox VNC/code surface.
- Actions hydration: yes, same Linux SSH contract as other SSH lease providers.
- Coordinator: no.

## Gotchas

- Run `devbox login` first. Crabbox does not store Namespace credentials.
- `builtin:base` is the current Namespace built-in base image. Avoid
  `default`; the current CLI treats that as a Docker image reference.
- Namespace Devboxes pause/resume outside Crabbox; stopped Devboxes keep their
  persistent storage.
- `--namespace-size` accepts `S`, `M`, `L`, or `XL`. Crabbox class mapping is
  `standard=S`, `fast=M`, `large=L`, and `beast=XL`.
- `namespace.repository` asks Namespace to clone a repo, but Crabbox still syncs
  the local dirty checkout into `namespace.workRoot`.

Related docs:

- [Feature: Namespace Devbox](../features/namespace-devbox.md)
- [Namespace Devbox setup](../features/namespace-devbox-setup.md)
- [Provider backends](../provider-backends.md)
