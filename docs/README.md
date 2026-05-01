# Crabbox Docs

Crabbox is a shared remote testbox system for OpenClaw. It gives maintainers a Blacksmith Testbox-like local workflow on owned machines: acquire a box, sync the current dirty tree, run tests remotely, stream output, and clean up safely.

Start here:

- [Architecture](architecture.md): components, lease flow, data model, and backends.
- [Orchestrator](orchestrator.md): coordinator behavior, leases, status, cleanup, and heartbeats.
- [CLI](cli.md): command surface, flags, config, output, and exit codes.
- [Commands](commands/README.md): one page per command.
- [MVP Plan](mvp-plan.md): what to build, in order.
- [Infrastructure](infrastructure.md): Cloudflare, Hetzner, DNS, Access, and fleet setup.
- [Security](security.md): auth, secrets, SSH, cleanup, and trust boundaries.
