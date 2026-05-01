# Features

Feature docs explain what Crabbox can do and how the pieces fit together. Command syntax lives in [../commands/README.md](../commands/README.md).

Read when:

- you want a capability overview;
- you are deciding where a behavior belongs;
- you need the feature-level contract before changing code.

Core features:

- [Coordinator](coordinator.md): brokered leases through Cloudflare Workers and Durable Objects.
- [Broker auth and routing](broker-auth-routing.md): bearer tokens, Cloudflare Access identity, and Worker routes.
- [Providers](providers.md): Hetzner and AWS EC2 Spot provisioning, classes, and fallback.
- [Runner bootstrap](runner-bootstrap.md): cloud-init, installed tools, SSH port, and readiness.
- [Sync](sync.md): Git seeding, rsync, fingerprints, excludes, and sanity checks.
- [SSH keys](ssh-keys.md): per-lease keys, provider key cleanup, and local storage.
- [Cost and usage](cost-usage.md): guardrails, provider-backed pricing, and reporting.
- [Lifecycle cleanup](lifecycle-cleanup.md): release, expiry, keep mode, and direct cleanup.
- [Repository onboarding](repository-onboarding.md): `crabbox init`, repo config, workflow stub, and agent skill.

Command docs:

- [doctor](../commands/doctor.md)
- [init](../commands/init.md)
- [warmup](../commands/warmup.md)
- [run](../commands/run.md)
- [status](../commands/status.md)
- [list](../commands/list.md)
- [usage](../commands/usage.md)
- [ssh](../commands/ssh.md)
- [inspect](../commands/inspect.md)
- [stop](../commands/stop.md)
- [cleanup](../commands/cleanup.md)
- [config](../commands/config.md)
