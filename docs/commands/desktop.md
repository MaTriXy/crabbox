# desktop

`crabbox desktop launch` starts an app inside a desktop lease without taking
over VNC manually.

```sh
crabbox warmup --desktop --browser
crabbox desktop launch --id blue-lobster --browser --url https://example.com
crabbox desktop launch --id blue-lobster --browser --url https://example.com --webvnc --open
crabbox desktop launch --id blue-lobster --browser --url https://discord.com/login --egress discord --webvnc --open
crabbox desktop launch --id blue-lobster -- xterm
```

The command resolves and touches the lease, verifies `desktop=true`, waits for
the loopback VNC service, then starts the process detached from the SSH session.
With `--browser`, Crabbox probes the target browser the same way `run --browser`
does and launches `BROWSER` when no explicit command is provided.
With `--webvnc`, the command keeps running after launch and bridges the desktop
into the authenticated WebVNC portal. Add `--open` to open that portal locally.
Browser launches default to a windowed human desktop with the remote panel and
title bar visible; use `--fullscreen` only for capture/video workflows.

`--egress <profile>` passes the active lease-local egress proxy to the launched
browser as `--proxy-server=http://127.0.0.1:3128`, so the browser exits to the
internet through the operator machine running `crabbox egress start`. Start
the egress bridge first; the flag currently requires `--browser`. Override the
proxy address with `--egress-proxy host:port` if you started egress on a
non-default port. See [egress](egress.md) for the full bridge model.

On Windows, SSH sessions cannot directly own the visible console desktop, so
Crabbox writes a one-shot PowerShell launcher under `C:\ProgramData\crabbox` and
runs it as an interactive scheduled task for the logged-in `crabbox` user. The
launcher minimizes existing windows, starts the app, and tries to foreground
the new process. On Linux and macOS, the command is detached with `setsid` or
`nohup`.

Flags:

```text
--id <lease-id-or-slug>
--provider hetzner|aws|ssh
--target linux|macos|windows
--windows-mode normal|wsl2
--static-host <host>
--static-user <user>
--static-port <port>
--static-work-root <path>
--browser
--url <url>
--webvnc
--open
--fullscreen
--egress <profile>
--egress-proxy <host:port>
--reclaim
```

Related docs:

- [egress](egress.md)
- [vnc](vnc.md)
- [webvnc](webvnc.md)
- [Lease capabilities](../features/capabilities.md)
- [Mediated egress](../features/egress.md)
