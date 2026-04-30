# Crabbox

Crabbox is an open source remote testbox runner for OpenClaw maintainers. It gives a Blacksmith Testboxes-style local loop on owned cloud capacity: provision or reuse a warm Linux box, sync the current dirty checkout, run a command remotely, stream output, and clean up.

The current implementation is a Go CLI plus a Cloudflare Worker/Durable Object coordinator. The CLI uses the coordinator for brokered Hetzner or AWS EC2 Spot leases, with direct provider calls kept as a debug fallback.

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
- AWS EC2 Spot provisioning with class fallback
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
- SSH key at `~/.ssh/id_ed25519`, or set `CRABBOX_SSH_KEY`
- broker config in `~/.config/crabbox/config.json` or `~/Library/Application Support/crabbox/config.json` on macOS

Build:

```sh
go build -o bin/crabbox ./cmd/crabbox
```

Configure the deployed broker:

```sh
printf '%s' "$CRABBOX_COORDINATOR_TOKEN" | \
  bin/crabbox config set-broker \
    --url https://crabbox-coordinator.steipete.workers.dev \
    --provider aws \
    --token-stdin
```

Check local prerequisites and broker access:

```sh
bin/crabbox doctor
```

Inspect broker config:

```sh
bin/crabbox config show
```

Warm a reusable OpenClaw testbox:

```sh
bin/crabbox warmup --profile openclaw-check --class beast --keep
```

Use AWS EC2 Spot through the broker:

```sh
bin/crabbox warmup --class beast --keep
```

Run a command on an existing lease:

```sh
CI=1 bin/crabbox run --id cbx_... -- pnpm test:changed:max
```

Stop a kept server:

```sh
bin/crabbox stop cbx_...
```

Print the CLI version:

```sh
bin/crabbox --version
```

## Machine Classes

`beast` is the default. Hetzner uses dedicated-server classes:

```text
standard  ccx33, cpx62, cx53
fast      ccx43, cpx62, cx53
large     ccx53, ccx43, cpx62, cx53
beast     ccx63, ccx53, ccx43, cpx62, cx53
```

During verification, Hetzner rejected `ccx63`, `ccx53`, and `ccx43` because of the account dedicated-core quota, so Crabbox fell back to `cpx62`.

AWS uses EC2 Spot C7a classes:

```text
standard  c7a.8xlarge, c7a.4xlarge
fast      c7a.16xlarge, c7a.12xlarge, c7a.8xlarge
large     c7a.24xlarge, c7a.16xlarge, c7a.12xlarge
beast     c7a.48xlarge, c7a.32xlarge, c7a.24xlarge, c7a.16xlarge
```

Set `CRABBOX_SERVER_TYPE` or pass `--type` to use another EC2 type such as `c8a.24xlarge`.

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
AWS_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY
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

Config file:

```json
{
  "broker": {
    "url": "https://crabbox-coordinator.steipete.workers.dev",
    "provider": "aws",
    "token": "..."
  },
  "class": "beast",
  "aws": {
    "region": "eu-west-1",
    "rootGB": 400
  },
  "ssh": {
    "key": "~/.ssh/id_ed25519",
    "user": "crabbox",
    "port": "2222"
  }
}
```

Environment variables remain supported for automation and direct-provider debug:

```text
HCLOUD_TOKEN or HETZNER_TOKEN     Hetzner Cloud API token
AWS_PROFILE/AWS_*                AWS SDK credentials for direct --provider aws fallback
CRABBOX_PROFILE                  default openclaw-check
CRABBOX_PROVIDER                 default hetzner
CRABBOX_CONFIG                   optional config file override
CRABBOX_COORDINATOR              optional broker URL override
CRABBOX_COORDINATOR_TOKEN        optional broker bearer token override
CRABBOX_DEFAULT_CLASS            default beast
CRABBOX_SERVER_TYPE              provider-specific override
CRABBOX_HETZNER_LOCATION         default fsn1
CRABBOX_HETZNER_IMAGE            default ubuntu-24.04
CRABBOX_HETZNER_SSH_KEY          default crabbox-steipete
CRABBOX_AWS_REGION               default eu-west-1
CRABBOX_AWS_AMI                  optional Ubuntu AMI override
CRABBOX_AWS_SECURITY_GROUP_ID    optional security group override
CRABBOX_AWS_SUBNET_ID            optional subnet override
CRABBOX_AWS_INSTANCE_PROFILE     optional IAM instance profile name
CRABBOX_AWS_ROOT_GB              default 400
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
goreleaser release --snapshot --clean --skip=publish
npm ci --prefix worker
npm run format:check --prefix worker
npm run lint --prefix worker
npm run check --prefix worker
npm test --prefix worker
npm run build --prefix worker
```

CI runs the same checks on pushes and pull requests.

## Releases

Tagged pushes matching `v*` publish Go CLI archives through GoReleaser. Manual reruns can use the `release` workflow with a tag input.

## Docs

- [docs/architecture.md](docs/architecture.md)
- [docs/cli.md](docs/cli.md)
- [docs/infrastructure.md](docs/infrastructure.md)
- [docs/mvp-plan.md](docs/mvp-plan.md)
- [docs/security.md](docs/security.md)

## License

Crabbox is released under the MIT License. See [LICENSE](LICENSE).
