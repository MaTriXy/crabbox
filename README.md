# Crabbox

Crabbox is an open source remote testbox runner for OpenClaw maintainers. It gives a Blacksmith Testboxes-style local loop on owned Hetzner capacity: provision or reuse a warm Linux box, sync the current dirty checkout, run a command remotely, stream output, and clean up.

The current implementation is a direct Go CLI that talks to Hetzner Cloud and SSH. The planned shared control plane is a Cloudflare Worker plus Durable Object coordinator; see [docs/mvp-plan.md](docs/mvp-plan.md).

## Status

Working today:

- `crabbox doctor`
- `crabbox warmup`
- `crabbox run`
- `crabbox stop`
- `crabbox pool list`
- `crabbox machine cleanup`
- Hetzner server provisioning with class fallback
- cloud-init bootstrap for Node 22, pnpm, Docker, Git, and rsync
- rsync overlay of local dirty worktrees
- shallow Git hydration for OpenClaw changed-test detection
- SSH execution on port `2222`

Not yet done:

- Cloudflare coordinator API
- Durable Object shared lease store
- `crabbox login`
- GitHub Actions/OIDC-compatible execution
- untrusted multi-tenant isolation

## Quick Start

Prerequisites:

- Go 1.26+
- `git`, `ssh`, `rsync`, and `curl`
- Hetzner token in `HCLOUD_TOKEN` or `HETZNER_TOKEN`
- SSH key at `~/.ssh/id_ed25519`, or set `CRABBOX_SSH_KEY`

Build:

```sh
go build -o bin/crabbox ./cmd/crabbox
```

Check local prerequisites and Hetzner access:

```sh
bin/crabbox doctor
```

Warm a reusable OpenClaw testbox:

```sh
bin/crabbox warmup --profile openclaw-check --class beast --keep
```

Run a command on an existing lease:

```sh
CI=1 bin/crabbox run --id cbx_... -- pnpm test:changed:max
```

Stop a kept server:

```sh
bin/crabbox stop cbx_...
```

## Machine Classes

`beast` is the default. It tries the biggest useful Hetzner machines first, then falls back if the account hits quota or capacity limits:

```text
standard  ccx33, cpx62, cx53
fast      ccx43, cpx62, cx53
large     ccx53, ccx43, cpx62, cx53
beast     ccx63, ccx53, ccx43, cpx62, cx53
```

During verification, Hetzner rejected `ccx63`, `ccx53`, and `ccx43` because of the account dedicated-core quota, so Crabbox fell back to `cpx62`.

## OpenClaw Verification

Verified from `/Users/steipete/Projects/openclaw` on a warm fallback `cpx62` runner:

```sh
CI=1 /usr/bin/time -p /Users/steipete/Projects/crabbox/bin/crabbox run --id cbx_f782c469c9ce -- pnpm test:changed:max
```

Result:

- 61 Vitest shards completed successfully.
- End-to-end wall time was 93.17 seconds.
- The timing includes rsync scan, remote Git hydration, command execution, and output streaming.

For true Blacksmith Testboxes parity, raise the Hetzner dedicated-core quota and re-run on `ccx63`.

## Configuration

Environment variables:

```text
HCLOUD_TOKEN or HETZNER_TOKEN     Hetzner Cloud API token
CRABBOX_PROFILE                  default openclaw-check
CRABBOX_DEFAULT_CLASS            default beast
CRABBOX_HETZNER_LOCATION         default fsn1
CRABBOX_HETZNER_IMAGE            default ubuntu-24.04
CRABBOX_HETZNER_SSH_KEY          default crabbox-steipete
CRABBOX_SSH_KEY                  default ~/.ssh/id_ed25519
CRABBOX_SSH_USER                 default crabbox
CRABBOX_SSH_PORT                 default 2222
CRABBOX_WORK_ROOT                default /work/crabbox
```

Forwarded environment is intentionally narrow:

- `OPENCLAW_*`
- `NODE_OPTIONS`
- `CI`

Do not pass secret values as command-line arguments. Keep provider tokens outside the repository.

## Development

Run the local gate:

```sh
gofmt -w $(git ls-files '*.go')
go vet ./...
go test -race ./...
go build -trimpath -o bin/crabbox ./cmd/crabbox
```

CI runs the same checks on pushes and pull requests.

## Docs

- [docs/architecture.md](docs/architecture.md)
- [docs/cli.md](docs/cli.md)
- [docs/infrastructure.md](docs/infrastructure.md)
- [docs/mvp-plan.md](docs/mvp-plan.md)
- [docs/security.md](docs/security.md)

## License

Crabbox is released under the MIT License. See [LICENSE](LICENSE).
