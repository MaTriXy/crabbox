# desktop

`crabbox desktop launch` starts an app inside a desktop lease without taking
over VNC manually.

```sh
crabbox warmup --desktop --browser
crabbox desktop launch --id blue-lobster --browser --url https://example.com
crabbox desktop launch --id blue-lobster --browser --url https://example.com --webvnc --open
crabbox desktop launch --id blue-lobster -- xterm
```

The command resolves and touches the lease, verifies `desktop=true`, waits for
the loopback VNC service, then starts the process detached from the SSH session.
With `--browser`, Crabbox probes the target browser the same way `run --browser`
does and launches `BROWSER` when no explicit command is provided.
With `--webvnc`, the command keeps running after launch and bridges the desktop
into the authenticated WebVNC portal. Add `--open` to open that portal locally.

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
--reclaim
```
