# Namespace Devbox

Read when:

- choosing `provider: namespace-devbox`;
- comparing Namespace Devbox with Blacksmith Testbox;
- debugging Namespace CLI lifecycle around Crabbox SSH sync/run.

`provider: namespace-devbox` creates or reuses Namespace Devboxes and exposes
them to Crabbox as Linux SSH leases. Namespace owns Devbox lifecycle and auth;
Crabbox owns the local checkout sync and command execution.

## Setup

Install and authenticate the Namespace Devbox CLI:

```sh
devbox login
```

Then select the provider:

```yaml
provider: namespace-devbox
namespace:
  image: builtin:base
  size: M
  workRoot: /workspaces/crabbox
```

## Commands

```sh
crabbox warmup --provider namespace-devbox --namespace-image builtin:base
crabbox run --provider namespace-devbox --id <slug> -- pnpm test
crabbox ssh --provider namespace-devbox --id <slug>
crabbox list --provider namespace-devbox
crabbox stop --provider namespace-devbox <slug>
```

## Provider Boundary

Namespace is similar to Blacksmith only at the product category level: both can
provide ready remote compute for agents. The Crabbox integration is different:

- Blacksmith Testbox is a delegated run provider. Blacksmith owns sync and
  command transport through `blacksmith testbox run`.
- Namespace Devbox is an SSH lease provider. Namespace owns create, generated
  SSH config, and list; Crabbox owns rsync, SSH execution, Actions hydration,
  and timing.

## Config Keys

- `namespace.image`: Devbox image, default `builtin:base`.
- `namespace.size`: Devbox size, `S`, `M`, `L`, or `XL`.
- `namespace.repository`: optional repo checkout for Namespace to clone.
- `namespace.site`: optional Namespace site.
- `namespace.volumeSizeGB`: optional persistent volume size.
- `namespace.autoStopIdleTimeout`: Namespace idle auto-stop duration.
- `namespace.workRoot`: Crabbox sync root, default `/workspaces/crabbox`.
- `namespace.deleteOnRelease`: delete on stop instead of shutdown.

Related docs:

- [Provider: Namespace Devbox](../providers/namespace-devbox.md)
- [Namespace Devbox setup](namespace-devbox-setup.md)
- [Providers](providers.md)
