# Prebaked Runner Images

Read when:

- creating or promoting Crabbox runner images;
- speeding up desktop/browser QA leases;
- deciding whether state belongs in a provider image, a warm lease, or a repo cache.

Prebaked images store machine capabilities, not scenario state.

## Where Images Live

Provider-owned image storage is the source of truth:

- AWS: AMIs plus their EBS snapshots live in the AWS account. `crabbox image
  promote` stores the selected AMI id in coordinator metadata so future AWS
  brokered leases can use it.
- Hetzner: snapshots/images live in the Hetzner project. Crabbox can already
  boot a configured image through `image`/`CRABBOX_HETZNER_IMAGE`, but
  create/promote lifecycle commands are not implemented for Hetzner yet.
- Blacksmith Testbox: images are owned by Blacksmith/GitHub runner
  infrastructure, not Crabbox.

Do not store image bytes in git, release artifacts, or coordinator durable
state. The coordinator should hold only the current provider image identifier,
promotion metadata, and enough tags to explain provenance.

## Bake Into Images

Good prebake contents:

- OS patches and base packages;
- SSH, Git, rsync, curl, jq, and readiness helpers;
- desktop/browser capabilities for `--desktop --browser` leases;
- screenshot and recording tools such as `scrot`, `ffmpeg`, `xdotool`, and VNC;
- Node 22, corepack/pnpm, build-essential, Python, and common native-addon
  headers when the image targets browser/channel QA;
- empty shared cache directories such as `/var/cache/crabbox/pnpm`.

Bad prebake contents:

- personal or CI secrets;
- browser profiles, Slack/Discord/WhatsApp login state, cookies, or OAuth
  tokens;
- repository checkouts, `node_modules`, built product `dist/`, or PR artifacts;
- one-off debugging files.

## Runtime Caches

Runtime caches belong outside the image:

- warm leases can keep `/var/cache/crabbox/pnpm` and browser profiles for
  short-lived operator sessions;
- GitHub Actions should cache candidate pnpm stores by lockfile and platform;
- product-specific runtime bundles and evidence artifacts belong in the repo
  workflow workspace, for example under `.artifacts/qa-e2e/...`;
- long-lived reusable volumes should be keyed by repo, lockfile, Node version,
  platform, and image id before Crabbox mounts them into leases.

This split keeps images reusable across repositories while still letting slow QA
systems skip repeated dependency work when they deliberately reuse a warm lease
or a keyed external cache.

Related docs:

- [image command](../commands/image.md)
- [Runner bootstrap](runner-bootstrap.md)
- [Interactive desktop and VNC](interactive-desktop-vnc.md)
