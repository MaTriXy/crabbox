# Interactive Desktop And VNC

Read when:

- adding or using browser/UI QA that needs a visible Linux desktop;
- deciding whether Mantis, OpenClaw, or Crabbox owns VNC setup;
- debugging an interactive QA lease that needs operator takeover.

Interactive desktop support belongs in Crabbox. Crabbox owns machine lifecycle,
network reachability, SSH keys, lease expiry, and provider-specific setup.
Scenario systems such as Mantis should ask for a desktop-capable lease and then
drive browser automation, screenshots, artifacts, and PR comments from inside
that lease.

The intended contract is:

- `crabbox warmup --desktop` leases or reuses a Linux machine with the normal
  Crabbox SSH contract plus a desktop profile;
- `crabbox vnc --id <lease>` prints a tunnel command and connection metadata for
  operator takeover;
- `crabbox run --id <lease> --desktop -- <command...>` runs UI automation in
  the desktop session;
- desktop services bind to loopback on the runner and are reachable through SSH
  tunnels only;
- screenshots, traces, videos, and browser profiles remain regular command
  artifacts owned by the caller or repository workflow.

Crabbox should provision the reusable machine capability:

- Xvfb or a lightweight compositor/display manager;
- a small window manager suitable for browser automation;
- Chromium or Chrome when the repository did not install one already;
- x11vnc or an equivalent VNC server bound to `127.0.0.1`;
- optional noVNC/websockify when browser-based takeover is needed;
- a persistent browser profile root under the lease work area.

Crabbox should not own product-specific scenario logic:

- provider tokens and app credentials;
- Discord, Slack, WhatsApp, email, or OpenClaw workflow setup;
- screenshots that prove a bug before and after a fix;
- PR comments or issue triage.

Those belong to Mantis or the repository workflow. Crabbox's job is to make the
machine debuggable and reproducible.

Security rules:

- never expose VNC directly to the public internet;
- prefer SSH local forwarding such as `localhost:5901 -> 127.0.0.1:5900`;
- generate per-lease VNC passwords only when a VNC server requires them;
- redact passwords from logs and run records;
- stop desktop services when the lease stops;
- keep the normal TTL and idle-timeout lifecycle in force.

Provider notes:

- Hetzner and AWS brokered Linux leases are the primary target because Crabbox
  controls cloud-init and firewall shape there.
- Static SSH Linux hosts can participate when the operator accepts responsibility
  for packages and display services.
- Blacksmith Testbox can run headless browser automation today, but VNC takeover
  needs a Blacksmith-supported SSH tunnel or connection-info API before Crabbox
  can offer the same `vnc` command there.
- macOS and Windows are static-host concerns, not first-pass Crabbox desktop
  provisioning.

For Mantis, the first consumer should be a Discord QA lane:

1. lease a desktop-capable Linux runner;
2. hydrate OpenClaw and the Discord bot credentials;
3. create a named browser profile;
4. reproduce the baseline and capture screenshots;
5. apply or check out the candidate fix;
6. rerun the same scenario and capture candidate screenshots;
7. attach artifacts and a compact visual summary to the PR.

Related docs:

- [Runner bootstrap](runner-bootstrap.md)
- [Providers](providers.md)
- [SSH keys](ssh-keys.md)
- [Actions hydration](actions-hydration.md)
