# Crabbox Docs

Crabbox is a shared remote testbox system for OpenClaw. It gives maintainers a Blacksmith Testbox-like local workflow on owned machines: acquire a box, sync the current dirty tree, run tests remotely, stream output, and clean up safely.

The GitHub Pages site is generated from these Markdown files by `scripts/build-docs-site.mjs`. Markdown remains the source of truth; the generated site lives in `dist/docs-site` during local builds and is deployed by `.github/workflows/pages.yml`.

Pages deploy uses GitHub Actions. The repository or organization must allow GitHub Pages before the workflow can publish.

Start here:

- [Architecture](architecture.md): components, lease flow, data model, and backends.
- [Orchestrator](orchestrator.md): coordinator behavior, leases, status, cleanup, and heartbeats.
- [CLI](cli.md): command surface, flags, config, output, and exit codes.
- [Commands](commands/README.md): one page per command.
- [Features](features/README.md): one page per feature area.
- [MVP Plan](mvp-plan.md): what to build, in order.
- [Infrastructure](infrastructure.md): Cloudflare, Hetzner, DNS, Access, and fleet setup.
- [Security](security.md): auth, secrets, SSH, cleanup, and trust boundaries.

Build the docs site locally:

```sh
node scripts/build-docs-site.mjs
open dist/docs-site/index.html
```
