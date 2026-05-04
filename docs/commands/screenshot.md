# screenshot

`crabbox screenshot` captures a PNG from a Linux desktop lease without opening a
VNC client.

```sh
crabbox warmup --desktop
crabbox screenshot --id blue-lobster
crabbox screenshot --id blue-lobster --output desktop.png
```

The command resolves and touches the lease like `crabbox ssh`, verifies that the
lease has `desktop=true`, waits for the loopback desktop/VNC service, then
streams a PNG over SSH from `DISPLAY=:99`.

If `--output` is omitted, Crabbox writes:

```text
crabbox-<slug-or-id>-screenshot.png
```

Screenshots are currently supported for Linux desktop leases. Static macOS and
Windows targets are existing host machines, not Crabbox-created desktops, so
`screenshot` rejects those targets instead of capturing your local or home-host
desktop by accident.

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
--output <path>
--reclaim
```
