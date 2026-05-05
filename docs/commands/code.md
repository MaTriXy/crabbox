# code

`crabbox code` bridges a code-server workspace for a Linux lease into the
authenticated coordinator portal.

```sh
crabbox warmup --code
crabbox code --id blue-lobster
crabbox code --id blue-lobster --open
```

## How It Works

Create or reuse a lease with `code=true`:

```sh
crabbox warmup --code
```

The Linux bootstrap installs `code-server` only for leases that request the
capability. `crabbox code` then resolves the lease, starts `code-server` on
runner loopback, opens an SSH tunnel, mints a short-lived bridge ticket, and
registers a local bridge with the coordinator.

The browser URL is lease-scoped:

```text
/portal/leases/<lease-id>/code/
```

The data path is:

```text
browser
  <-> coordinator /portal/leases/<lease>/code/
  <-> local crabbox code process
  <-> SSH tunnel
  <-> runner 127.0.0.1:8080
```

Keep the local `crabbox code` process running while using the editor. The
coordinator authenticates the browser through portal auth and authenticates the
local bridge with a one-use, short-lived ticket.

## Flags

```text
--id <lease-id-or-slug>
--provider hetzner|aws
--target linux
--network auto|tailscale|public
--local-port <port>
--open
--reclaim
```

## Limitations

- Coordinator-backed Linux leases are supported.
- Static SSH hosts, Windows, macOS, and Blacksmith Testbox are intentionally not
  supported by this portal bridge yet.
- `code-server` auth is disabled on the runner side because the trusted access
  boundary is the authenticated coordinator portal plus the local bridge.

## Troubleshooting

`lease ... was not created with code=true`

Warm a new lease with the code capability:

```sh
crabbox warmup --code
```

The portal shows a bridge command

The browser can reach the coordinator, but no local bridge is registered. Start
`crabbox code --id <lease>` locally and keep it running.
