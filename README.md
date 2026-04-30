# Crabbox

Crabbox is an open source remote testbox runner for OpenClaw maintainers. It gives a Blacksmith Testboxes-style local loop on owned Hetzner capacity: provision or reuse a warm Linux box, sync the current dirty checkout, run a command remotely, stream output, and clean up.

The current implementation is a Go CLI plus a Cloudflare Worker/Durable Object coordinator. The CLI can also fall back to direct Hetzner Cloud calls when `CRABBOX_COORDINATOR` is unset.

## Status

Working today:

- `crabbox doctor`
- `crabbox warmup`
- `crabbox run`
- `crabbox stop`
- `crabbox pool list`
- `crabbox machine cleanup`
- Cloudflare Worker coordinator on Workers/Durable Objects
- bearer-token coordinator auth for automation
- Cloudflare route for `crabbox.clawd.bot/*`
- Hetzner server provisioning with class fallback
- cloud-init bootstrap for Node 22, pnpm, Docker, Git, and rsync
- rsync overlay of local dirty worktrees
- shallow Git hydration for OpenClaw changed-test detection
- SSH execution on port `2222`

Not yet done:

- `crabbox login`
- GitHub Actions/OIDC-compatible execution
- untrusted multi-tenant isolation

## Quick Start

Prerequisites:

- Go 1.26+
- `git`, `ssh`, `rsync`, and `curl`
- Hetzner token in `HCLOUD_TOKEN` or `HETZNER_TOKEN`
- SSH key at `~/.ssh/id_ed25519`, or set `CRABBOX_SSH_KEY`
- deployed coordinator env in `CRABBOX_COORDINATOR` and `CRABBOX_COORDINATOR_TOKEN`

Build:

```sh
go build -o bin/crabbox ./cmd/crabbox
```

Check local prerequisites and Hetzner access:

```sh
bin/crabbox doctor
```

Use the deployed coordinator:

```sh
export CRABBOX_COORDINATOR=https://crabbox-coordinator.steipete.workers.dev
bin/crabbox pool list
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

## Cloudflare Deployment

Worker source lives in `worker/`.

Local checks:

```sh
npm ci --prefix worker
npm run format:check --prefix worker
npm run lint --prefix worker
npm run check --prefix worker
npm test --prefix worker
npm run build --prefix worker
```

Deploy:

```sh
export CLOUDFLARE_API_TOKEN="$CRABBOX_CLOUDFLARE_API_TOKEN"
export CLOUDFLARE_ACCOUNT_ID="$CRABBOX_CLOUDFLARE_ACCOUNT_ID"
npx wrangler deploy --config worker/wrangler.jsonc
```

Required Worker secrets:

```text
HETZNER_TOKEN
CRABBOX_SHARED_TOKEN
```

The Worker is deployed at:

```text
https://crabbox-coordinator.steipete.workers.dev
```

The Cloudflare route `crabbox.clawd.bot/*` is also attached and currently protected by Cloudflare Access.

## OpenClaw Verification

Verified from `/Users/steipete/Projects/openclaw` on a Cloudflare-created fallback `cpx62` runner:

```sh
CI=1 /usr/bin/time -p /Users/steipete/Projects/crabbox/bin/crabbox run --id cbx_f60f47cbc879 -- pnpm test:changed:max
```

Result:

- 61 Vitest shards completed successfully.
- End-to-end warm wall time was 93.66 seconds through the Cloudflare coordinator path.
- The timing includes rsync scan, remote Git hydration, command execution, and output streaming.

For true Blacksmith Testboxes parity, raise the Hetzner dedicated-core quota and re-run on `ccx63`.

## Configuration

Environment variables:

```text
HCLOUD_TOKEN or HETZNER_TOKEN     Hetzner Cloud API token
CRABBOX_PROFILE                  default openclaw-check
CRABBOX_COORDINATOR              optional coordinator URL
CRABBOX_COORDINATOR_TOKEN        optional coordinator bearer token
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
npm ci --prefix worker
npm run format:check --prefix worker
npm run lint --prefix worker
npm run check --prefix worker
npm test --prefix worker
npm run build --prefix worker
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
