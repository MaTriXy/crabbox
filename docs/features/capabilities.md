# Lease Capabilities

Read when:

- adding `--desktop`, `--browser`, or `--code` to a workflow;
- changing how Crabbox detects whether a lease can host a visible desktop;
- adding a new lease capability flag.

Lease capabilities are opt-in features that change what a managed runner can
do beyond running headless commands. They are a separate concept from the
provider feature set declared in `ProviderSpec.Features`: feature set says
"this provider can support a desktop"; lease capability says "this lease was
created with a desktop and exposes one right now".

## The Three Capabilities

```text
--desktop  visible desktop with a loopback VNC server
--browser  Chrome/Chromium installed and exported via $BROWSER and $CHROME_BIN
--code     code-server bound to a loopback port for portal/code bridging
```

All three default to off. They have to be requested at lease creation time
(`crabbox warmup --desktop`) and reused afterwards. A lease created without a
capability cannot grow it later.

## Selection And Validation

Capability flags follow a two-step validation:

1. **Provider feature check.** When the user sets a capability flag,
   `validateRequestedCapabilities` looks up the selected provider's
   `Spec.Features` and rejects the request if the matching feature
   (`FeatureDesktop`, `FeatureBrowser`, `FeatureCode`) is missing. Hetzner
   Linux supports all three; Blacksmith Testbox supports none.
2. **Lease label check.** When reusing a lease (`--id`),
   `enforceManagedLeaseCapabilities` checks the matching label
   (`desktop=true`, `browser=true`, `code=true`) on the existing lease. If
   the label is missing, Crabbox refuses with a hint to warm a new lease.

For static SSH targets, label enforcement is skipped because Crabbox does not
own the host. The capability is detected probe-by-probe instead - `--desktop`
on a static target probes the loopback VNC port; `--browser` on a static
target probes for Chrome and exports `BROWSER`/`CHROME_BIN` from what it
finds.

`--code` is currently restricted to managed Linux leases. The validator
rejects it for Windows, macOS, and static SSH.

## Desktop

When a managed Linux lease is created with `--desktop`, bootstrap installs:

- Xvfb (virtual framebuffer);
- a slim XFCE session;
- x11vnc bound to `127.0.0.1:5900`;
- a randomized VNC password at `/var/lib/crabbox/vnc.password`;
- screenshot tooling (`scrot`) and ffmpeg.

`crabbox vnc --id ...` opens an SSH tunnel to that loopback port. The user's
local VNC viewer talks through the tunnel and uses the password the CLI
fetches from `/var/lib/crabbox/vnc.password`. There is no public VNC port; the
loopback bind is the security boundary.

Static targets must already expose loopback VNC at `127.0.0.1:5900`. macOS
hosts can enable Screen Sharing; Windows hosts need a VNC server bound to
loopback (TightVNC works).

For per-OS detail and known limits, see:

- [Linux VNC](vnc-linux.md);
- [Windows VNC](vnc-windows.md);
- [macOS VNC](vnc-macos.md);
- [Interactive desktop and VNC](interactive-desktop-vnc.md).

When the run injects environment, Crabbox also sets:

```text
DISPLAY=:99
CRABBOX_DESKTOP=1
```

Tools that respect `DISPLAY` will draw onto the desktop the lease created.

## Browser

`--browser` adds a usable browser to the lease without dragging in a full QA
test environment.

On managed Linux:

- Google Chrome stable when available;
- Chromium fallback;
- native addon build helpers (`build-essential`, `libgbm-dev`,
  `libnss3-dev`, etc.) so dependency installs that compile against Chromium
  succeed.

On static targets, Crabbox probes for an existing browser and reports an
error if none is found. `requestedCapabilityEnv` shells out to the host:

- macOS: `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`;
- Windows: `chrome.exe` or `msedge.exe` from PATH or the standard install
  directories;
- Linux: `$BROWSER`, `$CHROME_BIN`, then `google-chrome`, `chromium`, or
  `chromium-browser` from PATH.

The detected path is exported into the run as:

```text
BROWSER=/path/to/browser
CHROME_BIN=/path/to/browser
CRABBOX_BROWSER=1
```

Test runners that read `BROWSER` or `CHROME_BIN` (Vitest, Playwright, etc.)
work without extra plumbing. If a browser is requested but no binary is
found, the run aborts before the command starts.

For browser QA where the remote service is sensitive to source IP (Discord
login, Slack workspace bootstrap, regional CDN behavior), pair `--browser`
with [mediated egress](egress.md). `crabbox egress start` opens a lease-local
proxy that exits to the internet through the operator machine, and `crabbox
desktop launch --egress <profile>` passes that proxy to Chrome.

## Code

`--code` provisions code-server on managed Linux leases:

- installs the binary at `/usr/local/bin/code-server`;
- binds to a loopback port (default `8080`);
- generates an auth token stored in coordinator state.

The portal and `crabbox code --id ...` open a code-server tab through the
authenticated portal bridge at `/portal/leases/{id-or-slug}/code/`. The bridge
proxies HTTP and WebSocket traffic to the loopback port; the code-server
auth token is injected by the bridge so the user does not see it. There is no
public code-server port.

Code is managed-Linux-only because the bridge depends on the lease shape and
the cloud-init that prepares the binary. Windows, macOS, and static SSH are
intentionally not supported today.

## Capability Labels

Managed lease records carry capability labels so list, status, and detail
pages can render the capability matrix without re-probing the host:

```text
desktop=true|false
browser=true|false
code=true|false
```

`enforceManagedLeaseCapabilities` reads these labels to gate `--desktop`,
`--browser`, and `--code` on `--id` reuse paths. The labels are written when
the lease is created and never flipped on a live lease.

## Composing Capabilities

Capabilities are independent - any combination is allowed where the
provider supports them:

```sh
crabbox warmup --desktop                     # desktop only
crabbox warmup --desktop --browser           # browser running on the desktop
crabbox warmup --desktop --browser --code    # full interactive box
crabbox warmup --browser                     # headless browser, no VNC
crabbox warmup --code                        # editor-only Linux lease
```

Capability bootstrap adds installation time. A bare lease is the fastest to
warm; a lease with all three takes the longest. Use the lightest combination
that satisfies the workflow.

## Static Targets

For static SSH hosts, capability validation degrades to probe-based detection:

- `--desktop`: probe `127.0.0.1:5900` over SSH; fail with a clear error if
  the port is not bound;
- `--browser`: probe for a browser binary using the OS-specific search list;
  fail if none found;
- `--code` is rejected (managed Linux only).

This is intentional. Crabbox is not responsible for installing software on
operator-owned static hosts; if the box does not expose the capability, the
run should not silently fall back.

Related docs:

- [warmup command](../commands/warmup.md)
- [run command](../commands/run.md)
- [vnc command](../commands/vnc.md)
- [webvnc command](../commands/webvnc.md)
- [code command](../commands/code.md)
- [egress command](../commands/egress.md)
- [Interactive desktop and VNC](interactive-desktop-vnc.md)
- [Mediated egress](egress.md)
- [Browser portal](portal.md)
