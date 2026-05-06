# Image Bake Runbook

Read when:

- baking a new Crabbox AWS image;
- promoting or rolling back the default AWS image;
- preparing a desktop/browser image for Mantis or other UI QA;
- checking whether state belongs in the image or in a warm lease.

This runbook is for trusted operators. Image commands need coordinator admin
auth and can create provider-side artifacts that cost money until cleaned up.

## Naming

Use names that identify owner, purpose, and UTC bake time:

```text
openclaw-crabbox-linux-desktop-browser-YYYYMMDD-HHMM
openclaw-mantis-linux-desktop-browser-YYYYMMDD-HHMM
```

Use a generic `openclaw-crabbox-*` image when the contents are useful to many
repositories. Use `openclaw-mantis-*` only when the image is specifically tuned
for OpenClaw Mantis QA.

## What To Bake

Bake machine capabilities:

- current OS security updates;
- SSH, Git, rsync, curl, jq, and readiness helpers;
- Xvfb/slim XFCE/VNC for desktop leases;
- Chrome/Chromium for browser leases;
- `ffmpeg`, `ffprobe`, `scrot`, `xdotool`, and other capture helpers;
- Node 22, npm, corepack, pnpm;
- build-essential, Python, and common native-addon headers;
- empty cache directories such as `/var/cache/crabbox/pnpm`.

Do not bake scenario state:

- secrets, tokens, or provider credentials;
- browser profiles, cookies, Slack/Discord/WhatsApp sessions, or OAuth state;
- source checkouts, `node_modules`, `dist`, PR artifacts, screenshots, or
  videos;
- local operator notes or one-off debugging files.

## Create A Candidate AMI

Warm a source lease:

```bash
crabbox warmup \
  --provider aws \
  --class standard \
  --desktop \
  --browser \
  --ttl 2h \
  --idle-timeout 30m
```

Capture the lease id from the output. Use the canonical `cbx_...` id for image
commands, not only the friendly slug.

Verify the source lease:

```bash
crabbox run \
  --provider aws \
  --id <cbx_id> \
  --no-sync \
  --shell -- \
  'set -euo pipefail
   command -v ssh
   command -v git
   command -v rsync
   command -v jq
   command -v node
   command -v pnpm
   command -v ffmpeg
   command -v scrot
   command -v x11vnc
   command -v google-chrome || command -v chromium || command -v chromium-browser
   test -d /work/crabbox
   sudo mkdir -p /var/cache/crabbox/pnpm
   sudo chmod 1777 /var/cache/crabbox /var/cache/crabbox/pnpm'
```

Create the candidate image:

```bash
crabbox image create \
  --id <cbx_id> \
  --name openclaw-crabbox-linux-desktop-browser-YYYYMMDD-HHMM \
  --wait \
  --json
```

Keep the JSON output. At minimum, record the AMI id, name, source lease id,
creation time, and operator.

## Smoke Candidate Before Promotion

Boot the candidate explicitly. Use the provider image override supported by the
current environment, for example:

```bash
CRABBOX_AWS_AMI=ami-1234567890abcdef0 \
crabbox warmup \
  --provider aws \
  --class standard \
  --desktop \
  --browser \
  --ttl 30m \
  --idle-timeout 10m
```

Run a smoke on the candidate:

```bash
crabbox run \
  --provider aws \
  --id <candidate-cbx_id-or-slug> \
  --no-sync \
  --shell -- \
  'set -euo pipefail
   echo image-smoke-ok
   uname -srm
   command -v node
   command -v pnpm
   command -v ffmpeg
   command -v scrot
   command -v google-chrome || command -v chromium || command -v chromium-browser
   test -d /work/crabbox'
```

For Mantis images, also run a real desktop/browser proof:

```bash
crabbox screenshot --provider aws --id <candidate-cbx_id-or-slug> --output /tmp/crabbox-image-smoke.png
```

Do not promote if SSH readiness, browser startup, screenshot capture, or the
package/tool checks fail.

## Promote

Promote only after a candidate smoke passes:

```bash
crabbox image promote ami-1234567890abcdef0 --json
```

Then verify a normal brokered lease without overrides uses the promoted image:

```bash
crabbox warmup \
  --provider aws \
  --class standard \
  --desktop \
  --browser \
  --ttl 30m \
  --idle-timeout 10m

crabbox run \
  --provider aws \
  --id <new-cbx_id-or-slug> \
  --no-sync \
  --shell -- \
  'echo promoted-image-smoke-ok && command -v ffmpeg && command -v node'
```

Keep the previous promoted AMI available until at least one normal brokered
lease and one relevant QA lane pass on the new image.

## Roll Back

Rollback is another promotion:

```bash
crabbox image promote ami-previous-good --json
```

Run the normal brokered smoke again. Do not delete the failed AMI immediately;
keep it long enough to inspect tags, logs, and source-lease details.

## Cleanup

Promotion does not delete old AMIs or EBS snapshots. Cleanup is a provider
operator task:

- keep the current promoted AMI;
- keep the previous known-good AMI until the new one has real QA proof;
- deregister stale failed/candidate AMIs after investigation;
- delete their orphaned EBS snapshots in the AWS account.

Do not rely on Crabbox coordinator state as the source of truth for old image
storage costs. Check AWS directly.

## Hetzner Status

Hetzner image bytes belong in the Hetzner project. Crabbox can boot a configured
image through `image` or `CRABBOX_HETZNER_IMAGE`, but Hetzner image
create/promote lifecycle commands are not implemented yet. Until then, create
and manage Hetzner snapshots with Hetzner tooling, then configure Crabbox to use
the selected image.

Related docs:

- [Prebaked runner images](prebaked-images.md)
- [image command](../commands/image.md)
- [Runner bootstrap](runner-bootstrap.md)
- [Interactive desktop and VNC](interactive-desktop-vnc.md)
