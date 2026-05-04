# screenshot

`crabbox screenshot` captures a PNG from a desktop lease without opening a VNC
client.

```sh
crabbox warmup --desktop
crabbox screenshot --id blue-lobster
crabbox screenshot --id blue-lobster --output desktop.png
```

The command resolves and touches the lease like `crabbox ssh`, verifies that the
lease has `desktop=true`, waits for the loopback desktop/VNC service, then
streams a PNG over SSH. Linux captures `DISPLAY=:99`, Windows captures the
primary desktop with PowerShell/.NET drawing APIs, and macOS uses
`screencapture`.

If `--output` is omitted, Crabbox writes:

```text
crabbox-<slug-or-id>-screenshot.png
```

Static macOS and Windows targets are existing host machines, not Crabbox-created
desktops, so `screenshot` rejects those targets instead of capturing your local
or home-host desktop by accident. Managed AWS Windows and AWS macOS desktop
leases are Crabbox-created boxes and can be captured by lease id or slug.

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
