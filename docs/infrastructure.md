# Infrastructure

## Current Intended Setup

Primary public endpoint:

```text
https://crabbox.openclaw.ai
```

Current deployable Cloudflare fallback:

```text
https://crabbox.clawd.bot
```

Reason for the fallback: `openclaw.ai` currently resolves through Namecheap nameservers and is not visible as a Cloudflare zone in the available Cloudflare account. Cloudflare Workers custom domains are simplest when the zone is managed by Cloudflare.

## Cloudflare

Use Cloudflare for:

- HTTPS coordinator.
- Access auth.
- Worker runtime.
- Durable Object lease state.
- DNS/custom domain once the target zone is available.

Known setup:

- Access org: `openclaw-crabbox.cloudflareaccess.com`.
- Access enabled.
- Current IdPs: one-time PIN and GitHub.
- GitHub IdP name: `GitHub OpenClaw`.
- GitHub IdP restriction: org `openclaw`.
- Fallback Access app: `Crabbox Coordinator` on `crabbox.clawd.bot`.
- Fallback Access policy readback verifies the GitHub org include rule for `openclaw`.

Required env:

```text
CRABBOX_CLOUDFLARE_API_TOKEN
CRABBOX_CLOUDFLARE_ACCOUNT_ID
CRABBOX_CLOUDFLARE_ZONE_ID
CRABBOX_CLOUDFLARE_ZONE_NAME
CRABBOX_DOMAIN
CRABBOX_FALLBACK_DOMAIN
CRABBOX_GITHUB_ALLOWED_ORG
```

GitHub IdP needs a GitHub OAuth app:

```text
GitHub org: openclaw
App name: Crabbox Access
Homepage URL: https://crabbox.openclaw.ai
Callback URL: https://openclaw-crabbox.cloudflareaccess.com/cdn-cgi/access/callback
```

Store resulting values outside the repo:

```text
CRABBOX_GITHUB_OAUTH_CLIENT_ID
CRABBOX_GITHUB_OAUTH_CLIENT_SECRET
```

Current local status:

- Core Cloudflare, Hetzner, and GitHub tokens are present in local `~/.profile`.
- The Crabbox Cloudflare token is mirrored to MacBook Pro `~/.profile`.
- `CRABBOX_COORDINATOR` and `CRABBOX_COORDINATOR_TOKEN` are present in local and MacBook Pro `~/.profile`.
- GitHub OAuth client ID and secret are present in local and MacBook Pro `~/.profile`.
- Cloudflare Access GitHub IdP is created.
- Cloudflare Access fallback app is created for `crabbox.clawd.bot`.
- `CRABBOX_COORDINATOR`, `CRABBOX_PROFILE`, `CRABBOX_CONFIG`, `CRABBOX_FLEET_CONFIG`, `CRABBOX_SSH_KEY`, `CRABBOX_NO_COLOR`, and `CRABBOX_LOG` are optional CLI defaults and are not required to build the MVP.

The Cloudflare token `crabbox-deploy` is scoped to `Steipete@gmail.com's Account` and the `clawd.bot` zone. It verifies access to Workers scripts, Access applications, Access identity providers, Access keys, DNS records, and zone Worker routes from both the local machine and MacBook Pro.

## DNS Decision

Preferred path:

1. Add `openclaw.ai` to Cloudflare.
2. Copy existing DNS records exactly.
3. Add `crabbox.openclaw.ai`.
4. Switch nameservers at registrar.
5. Deploy Worker custom domain.

Temporary path:

1. Deploy Worker under `crabbox.clawd.bot`.
2. Keep `CRABBOX_DOMAIN=crabbox.openclaw.ai` as intended target.
3. Use fallback domain for early testing.
4. Move to `openclaw.ai` once DNS is ready.

## Hetzner

Use Hetzner Cloud for worker machines.

Required env:

```text
HCLOUD_TOKEN
HETZNER_TOKEN
```

MVP defaults:

```yaml
provider: hetzner-main
location: fsn1
serverType: ccx63
image: ubuntu-24.04
sshUser: crabbox
sshPort: 2222
workdir: /work/crabbox
```

Machine labels:

```text
crabbox=true
profile=openclaw-check
class=ccx33
lease=cbx_...
owner=<github-login-or-email>
ttl=<timestamp>
```

Current direct-CLI status:

- `crabbox warmup --profile openclaw-check --class beast --keep` provisions through the Hetzner API without requiring `hcloud`.
- The `beast` class tries `ccx63`, `ccx53`, `ccx43`, `cpx62`, then `cx53`.
- Dedicated-core types currently fail on the available account quota, so the verified runner used `cpx62`.
- Cloud-init installs Node 24, pnpm via corepack, Docker, Git, rsync, and a readiness probe through a retrying bootstrap script. This is required because AWS Ubuntu mirrors can transiently return 503 during Docker dependency installation.
- SSH prefers port 2222 and falls back to port 22 during AWS bootstrap when the base image exposes default SSH before the custom port restart lands.
- The verified kept lease was `cbx_f782c469c9ce` on server `128694755`, `cpx62`, `188.245.91.84`.

## AWS EC2 Spot

Use AWS as the first non-Hetzner burst backend. The Cloudflare coordinator brokers AWS EC2 Spot by default; the CLI direct provider remains available with `--provider aws` when no broker is configured.

Brokered AWS credentials live as Worker secrets:

```text
AWS_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY
AWS_SESSION_TOKEN optional
```

Direct fallback env is whatever the AWS SDK can resolve, such as:

```text
AWS_PROFILE
AWS_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY
AWS_SESSION_TOKEN
```

AWS-specific Crabbox env:

```text
CRABBOX_AWS_REGION               default eu-west-1
CRABBOX_AWS_AMI                  optional Ubuntu 24.04 x86_64 AMI override
CRABBOX_AWS_SECURITY_GROUP_ID    optional security group override
CRABBOX_AWS_SUBNET_ID            optional subnet override
CRABBOX_AWS_INSTANCE_PROFILE     optional IAM instance profile name
CRABBOX_AWS_ROOT_GB              default 400
```

The AWS provider imports the local SSH public key as an EC2 key pair when needed, creates or reuses a `crabbox-runners` security group when no security group is supplied, launches one-time Spot instances, tags instances and volumes with Crabbox lease metadata, and terminates non-kept instances after the command.

## Machine Classes

Fleet config should define machine classes instead of hardcoding provider types. Current Hetzner direct defaults:

```yaml
classes:
  standard:
    provider: hetzner-main
    serverTypes: [ccx33, cpx62, cx53]
    cpu: 8
    memory: 32gb
  fast:
    provider: hetzner-main
    serverTypes: [ccx43, cpx62, cx53]
    cpu: 16
    memory: 64gb
  large:
    provider: hetzner-main
    serverTypes: [ccx53, ccx43, cpx62, cx53]
    cpu: 32
    memory: 128gb
  beast:
    provider: hetzner-main
    serverTypes: [ccx63, ccx53, ccx43, cpx62, cx53]
    cpu: 48
    memory: 192gb
```

Current AWS defaults:

```yaml
classes:
  standard:
    provider: aws
    serverTypes: [c7a.8xlarge, c7a.4xlarge]
  fast:
    provider: aws
    serverTypes: [c7a.16xlarge, c7a.12xlarge, c7a.8xlarge]
  large:
    provider: aws
    serverTypes: [c7a.24xlarge, c7a.16xlarge, c7a.12xlarge]
  beast:
    provider: aws
    serverTypes: [c7a.48xlarge, c7a.32xlarge, c7a.24xlarge, c7a.16xlarge]
```

Profiles choose a default class, and commands can override with `--class`.

## Fleet Repo

`openclaw/crabbox-fleet` should contain:

```text
fleet.yaml
profiles/openclaw.yaml
bootstrap/cloud-init.yaml
images/README.md
```

It should not contain secrets or live lease data.

Example:

```yaml
version: 1
fleet:
  name: openclaw
  coordinator: https://crabbox.openclaw.ai

profiles:
  openclaw-check:
    labels: [linux, x64, docker, node24]
    defaultClass: fast
    ttl: 90m
    maxTTL: 24h
    sync:
      exclude: [node_modules, .turbo, .git/lfs]
    envAllowlist:
      - OPENCLAW_*
      - NODE_OPTIONS
```

## Deployment

MVP deploy command:

```sh
crabbox admin deploy-coordinator
```

Or a script first:

```sh
scripts/deploy-worker
```

Deployment should:

1. Build Worker.
2. Create/update Durable Object bindings.
3. Set Worker secrets.
4. Deploy Worker.
5. Verify `/v1/health` on `workers.dev`.
6. Configure route/custom domain on `crabbox.clawd.bot`.
7. Verify `/v1/health` on the fallback domain.

Use `npx wrangler` from the Worker package unless `wrangler` is installed globally. Do not assume `hcloud` is installed; the implementation can use the Hetzner API directly from Go or from the Worker.

Current deployed coordinator:

```text
https://crabbox-coordinator.steipete.workers.dev
crabbox.clawd.bot/* -> crabbox-coordinator, protected by Cloudflare Access
```

Current Worker secrets:

```text
HETZNER_TOKEN
CRABBOX_SHARED_TOKEN
```

## Verified OpenClaw Run

Warm-run command from `/Users/steipete/Projects/openclaw` through the Cloudflare coordinator:

```sh
CI=1 /usr/bin/time -p /Users/steipete/Projects/crabbox/bin/crabbox run --id cbx_f60f47cbc879 -- pnpm test:changed:max
```

Result:

- 61 Vitest shards completed successfully.
- End-to-end warm wall time: 93.66 seconds.
- Runner class: requested `beast`, actual fallback `cpx62`.
- Sync path: rsync overlay plus remote Git hydrate for shallow checkout merge-base support.

## Local, MacBook Pro, And Mac Studio

The same required env should exist on the local machine, MacBook Pro, and Mac Studio. Do not commit these values.
