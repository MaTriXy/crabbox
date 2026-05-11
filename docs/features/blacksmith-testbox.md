# Blacksmith Testbox

Read when:

- choosing a provider/service page;
- choosing `provider: blacksmith-testbox`;
- changing Blacksmith CLI forwarding;
- deciding what Crabbox owns versus Blacksmith owns.

Crabbox can use Blacksmith Testboxes as the machine backend without using the Crabbox broker. Select it with `--provider blacksmith-testbox` for one command, or put `provider: blacksmith-testbox` in config when a repo or machine should use it by default.

Blacksmith is a delegated service integration. Crabbox does not provision,
bootstrap, sync, or expose VNC for the Testbox itself; it forwards to the
Blacksmith CLI and keeps local Crabbox ergonomics around that CLI.

## One-Liners

If you already have a Blacksmith Testbox ID, no Crabbox YAML is required:

```sh
crabbox run --provider blacksmith-testbox --id tbx_123 -- pnpm test
```

If Crabbox has already claimed a friendly slug for that Testbox, the slug works too:

```sh
crabbox run --provider blacksmith-testbox --id blue-lobster -- pnpm test:changed
crabbox status --provider blacksmith-testbox --id blue-lobster
crabbox stop --provider blacksmith-testbox blue-lobster
```

That path only needs Blacksmith auth and a reachable Testbox. Crabbox resolves the ID or slug, preserves the local repo claim, forwards the command to `blacksmith testbox run`, and prints `sync=delegated` in the final summary.

To create a fresh Testbox without YAML, provide the workflow details as flags:

```sh
crabbox warmup \
  --provider blacksmith-testbox \
  --blacksmith-org openclaw \
  --blacksmith-workflow .github/workflows/ci-check-testbox.yml \
  --blacksmith-job test \
  --blacksmith-ref main \
  --idle-timeout 90m
```

The same flags work for one-shot `run` when no `--id` is supplied:

```sh
crabbox run \
  --provider blacksmith-testbox \
  --blacksmith-workflow .github/workflows/ci-check-testbox.yml \
  --blacksmith-job test \
  -- pnpm test
```

YAML is a convenience, not a requirement, when the command line already tells Crabbox which backend and workflow to use. Environment variables such as `CRABBOX_BLACKSMITH_WORKFLOW`, `CRABBOX_BLACKSMITH_JOB`, `CRABBOX_BLACKSMITH_REF`, and `CRABBOX_BLACKSMITH_ORG` are also supported for shell defaults or scripts.

## Repo Config

Use repo config when every agent or maintainer should get the same Blacksmith defaults without repeating flags:

```yaml
provider: blacksmith-testbox
blacksmith:
  org: openclaw
  workflow: .github/workflows/ci-check-testbox.yml
  job: test
  ref: main
  idleTimeout: 90m
```

For repos that already use Crabbox Actions hydration, `blacksmith.workflow`, `blacksmith.job`, and `blacksmith.ref` can be omitted when `actions.workflow`, `actions.job`, and `actions.ref` carry the same values.

`blacksmith` is accepted as a shorthand provider alias, but docs and scripts should prefer `blacksmith-testbox`.

## Forwarded Commands

Crabbox forwards lifecycle and run operations to the Blacksmith CLI:

```sh
blacksmith testbox warmup <workflow> --job <job> --ref <ref> --ssh-public-key <key> --idle-timeout <minutes>
blacksmith testbox run --id <tbx_id> --ssh-private-key <key> <command>
blacksmith testbox list
blacksmith testbox list --all
blacksmith testbox stop --id <tbx_id>
```

The wrapper is deliberately thin for warmup, run, and stop. `crabbox list` and
`crabbox status` normalize Blacksmith data into Crabbox's common list/status
views so rendering stays core-owned across providers. Status currently reads
`blacksmith testbox list --all` to build that view.

If `blacksmith testbox list --all` and `crabbox status --provider
blacksmith-testbox --id <tbx_id>` work but new warmups remain `queued` with no
IP, treat it as Blacksmith service, queue, org-limit, or billing pressure
instead of a Crabbox provisioning bug. Stop queued IDs you created and switch to
another provider until the Blacksmith account or service recovers.

`crabbox list --provider blacksmith-testbox --json` parses the Blacksmith table
output into compatibility JSON rows with the fields Crabbox can see. That parser is a
compatibility layer, not a Blacksmith API contract. If the Blacksmith CLI adds
native JSON output, Crabbox should switch to that and drop table parsing.

When coordinator auth is configured, `crabbox list --provider blacksmith-testbox`
also performs a best-effort sync of the current all-status Blacksmith list into
the portal lease table. Those muted rows are owner-scoped visibility records for
Blacksmith-owned Testboxes. When the row includes enough context, Crabbox queries
GitHub Actions and links the row to the closest workflow run plus the workflow
definition. The portal also renders the Actions status/conclusion, adds a
`stuck` filter for long-queued or long-running workflow owners, and offers a
copyable local `crabbox stop --provider blacksmith-testbox ...` command for
operator cleanup. Clicking the row opens a visibility-only detail page with
owner/org, Actions ownership, timestamps, boundary notes, and the same local stop
command. They are not Crabbox leases, do not expose box access actions, do not
heartbeat, do not participate in Crabbox expiry or cost control, and become stale
when a later sync does not see the runner.

## Auth

Auth stays with Blacksmith. Run `blacksmith auth login` before using this provider. Crabbox does not call the Crabbox login broker, does not send work to the Cloudflare coordinator, and does not hold Blacksmith credentials.

## Ownership Boundary

- Blacksmith owns provisioning, workflow hydration, remote workspace setup, sync, command transport, logs emitted by its CLI, and idle expiry.
- Crabbox owns local YAML/env config, per-Testbox SSH keys, friendly slugs, repo claims, provider selection, command quoting, and final timing summaries.

Because Blacksmith owns sync in this mode, Crabbox sync flags such as
`--sync-only`, `--checksum`, `--force-sync-large`, and sync guardrails do not
apply. Crabbox also rejects SSH-run-only features such as `--script`,
`--script-stdin`, `--fresh-pr`, local stdout/stderr captures,
`--capture-on-fail`, and `--download`. `crabbox run` prints `sync=delegated` in
the final summary. Failed delegated runs still save a local bundle with
stdout/stderr, timing, and redacted env/config metadata. `--keep-on-failure` is
supported for one-shot Testbox runs so failed boxes can remain inspectable until
the Testbox idle timeout or explicit `crabbox stop`.

`blacksmith.workflow` is required only when Crabbox needs to warm or acquire a Testbox. Reusing an existing `tbx_...` ID or slug does not need workflow config.

## Desktop And VNC

Blacksmith can run headless browser automation through its own runner setup, but
Crabbox does not currently expose `crabbox vnc`, `crabbox webvnc`, or managed
screenshots for `provider=blacksmith-testbox`. Blacksmith owns machine
connectivity in this mode. Crabbox should add VNC only after Blacksmith exposes
a stable SSH tunnel or connection-info API that preserves the same security
boundary as managed Crabbox leases.

## Choosing The Path

Use the one-liner when:

- you already have `tbx_...`;
- you are trying Blacksmith on one command;
- an agent can pass provider and workflow directly as flags.

Use repo YAML when:

- the repo should default to Blacksmith;
- multiple agents should share the same workflow/job/ref;
- you want `crabbox warmup` to work without extra env.

Related docs:

- [Providers](providers.md)
- [Interactive desktop and VNC](interactive-desktop-vnc.md)
- [run command](../commands/run.md)
- [warmup command](../commands/warmup.md)
- [Source map](../source-map.md)
