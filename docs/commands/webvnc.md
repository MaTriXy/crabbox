# webvnc

`crabbox webvnc` bridges a desktop lease into the authenticated coordinator
portal.

Use it when you want the same VNC desktop that `crabbox vnc` opens, but inside
a browser tab instead of a native VNC client.

```sh
crabbox warmup --desktop
crabbox webvnc --id blue-lobster
crabbox webvnc --id blue-lobster --network tailscale
crabbox webvnc --id blue-lobster --open
crabbox webvnc --id blue-lobster --daemon --open
```

## How It Works

The command resolves the lease like `crabbox vnc`, verifies that the lease has
`desktop=true`, starts the normal SSH tunnel to the runner's loopback VNC
service, mints a short-lived bridge ticket over the authenticated coordinator
API, and opens a websocket bridge to the coordinator with that ticket. The
browser connects to `/portal/leases/<lease>/vnc` after GitHub portal auth, and
the Durable Object pairs that browser websocket with the local bridge process.

The data path is:

```text
browser noVNC
  <-> coordinator portal websocket
  <-> local crabbox webvnc process
  <-> SSH tunnel
  <-> runner 127.0.0.1:5900
```

That means the local `crabbox webvnc` process is not just a launcher. It is the
live bridge between the browser and the SSH-tunneled VNC socket. Keep it
running while the browser tab is open. If the browser tab reloads or drops, the
command re-registers a fresh bridge so the portal retry can reconnect.

## Security Boundary

This keeps the security boundary the same as `crabbox vnc`:

- VNC stays bound to runner loopback.
- The cloud provider does not open public VNC ingress.
- The coordinator authenticates the browser through portal auth and the bridge
  through a one-use short-lived ticket.
- The noVNC client is served from the coordinator origin, not a third-party CDN.
- The local `crabbox webvnc` process must keep running while the browser uses
  the desktop.

Use `--daemon` (or `--background`) to keep the bridge running without a tmux or
foreground shell. Crabbox writes the bridge log and pid file under its local
state directory and prints both paths. Use `--status` to print those paths
again, and `--stop` to kill the background bridge for that lease. Shutdown
terminates both the daemon supervisor and the active child bridge process.

`--network tailscale` changes only the SSH endpoint used for the local tunnel.
The runner VNC service stays bound to loopback.

## Portal And Passwords

`--open` opens the portal page after the bridge starts. If the VNC password is
available, the command also places it in the URL fragment for the local browser
tab. URL fragments are not sent to the coordinator, and Crabbox preserves
special characters such as `!` when building the fragment. If the portal login
flow redirects first, the page may still prompt for the VNC password; use the
password printed by the command. If an old browser tab is retrying with a stale
fragment, close it before opening the new bridge URL.

The portal page may show `waiting for bridge` until the local command has
connected. If you opened the portal first, start:

```sh
crabbox webvnc --id <lease-id-or-slug>
```

in a terminal and leave it running.

## Flags

Flags:

```text
--id <lease-id-or-slug>
--provider hetzner|aws
--target linux|macos|windows
--windows-mode normal|wsl2
--static-host <host>
--static-user <user>
--static-port <port>
--static-work-root <path>
--network auto|tailscale|public
--local-port <port>
--open
--daemon
--background
--status
--stop
--reclaim
```

## Limitations

Limitations:

- Coordinator-backed Hetzner and AWS desktop leases are supported.
- Static SSH hosts are intentionally not supported yet because the portal cannot
  prove that host-managed VNC credentials and prompts are safe to expose.
- Blacksmith Testbox still owns its own machine connectivity.

## Troubleshooting

`webvnc requires a configured coordinator login`

Run `crabbox login` for the coordinator you are using. WebVNC needs both the CLI
bridge and the browser portal to authenticate with the coordinator.

`webvnc currently supports coordinator-backed hetzner/aws desktop leases`

WebVNC is not available for static SSH hosts or Blacksmith Testbox. Use
`crabbox vnc` for static hosts when you explicitly trust the host-managed VNC
service.

`target does not expose VNC on 127.0.0.1:5900`

The lease is reachable over SSH, but the desktop service is not ready or was not
provisioned. Create the lease with `--desktop`, or wait for bootstrap to finish
and retry.

The portal keeps saying `waiting for bridge`

The browser can reach the coordinator, but no local bridge is currently paired
with that lease. Start or restart `crabbox webvnc --id <lease>` locally and keep
the process running. If the command is still running, wait for the portal retry
or reload the browser tab.

VNC authentication fails

Use the password printed by `crabbox webvnc`. With `--open`, the command tries
to pass the password in the browser URL fragment, but a portal login redirect
can lose that fragment before noVNC sees it.

Related docs:

- [Interactive desktop and VNC](../features/interactive-desktop-vnc.md)
- [Linux VNC](../features/vnc-linux.md)
- [Windows VNC](../features/vnc-windows.md)
- [macOS VNC](../features/vnc-macos.md)
