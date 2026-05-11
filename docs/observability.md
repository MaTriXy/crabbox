# Observability

Read when:

- debugging a failed or slow run;
- checking who used capacity this month;
- finding a remote machine for SSH inspection;
- correlating Actions hydration with the remote workspace.

Crabbox exposes operational visibility through CLI commands, coordinator usage summaries, retained run history/logs, provider labels, GitHub Actions run links, and Worker logs. The reliable path is to keep the lease ID and run ID together.

## Lease State

Use `status`, `list`, and `inspect`:

```sh
bin/crabbox status --id blue-lobster
bin/crabbox list --json
bin/crabbox inspect --id blue-lobster --json
```

Important fields:

- lease ID and slug;
- owner and org;
- provider and server type;
- state;
- `createdAt`, `lastTouchedAt`, `idleTimeoutSeconds`, `ttlSeconds`, and `expiresAt`;
- public address;
- SSH user and port;
- keep/delete behavior.

Provider machines are labeled with Crabbox metadata so cloud consoles can be correlated back to the lease.

## Usage And Cost

Use `usage` for monthly summaries:

```sh
bin/crabbox usage
bin/crabbox usage --scope user --user alice@example.com
bin/crabbox usage --scope org --org example-org
bin/crabbox usage --scope all --json
```

Reports include lease count, active lease count, elapsed runtime, estimated elapsed cost, reserved worst-case cost, and breakdowns by owner, org, provider, and server type.

## Run History And Logs

Coordinator-backed `crabbox run` creates a durable run record before leasing
starts, appends lifecycle events while the CLI progresses, and finishes the run
with exit code, timing, and retained command output.

Use:

```sh
bin/crabbox history
bin/crabbox history --lease cbx_...
bin/crabbox history --owner alice@example.com --json
bin/crabbox events run_...
bin/crabbox attach run_...
bin/crabbox logs run_...
bin/crabbox results run_...
```

History is for command debugging, not unlimited log archival. Events are ordered
phase and output chunks for reconnect/inspection, and `attach` can follow those
events while the original CLI is still alive. Logs are bounded retained remote
stdout/stderr captures. `run --capture-stdout <path>` stores stdout only in the
local file and leaves coordinator logs/events to stderr plus lifecycle events.
`run --capture-stderr <path>` does the same for remote stderr. `run
--capture-on-fail` writes a local `.crabbox/captures/*.tar.gz` bundle after a
non-zero exit; Crabbox does not redact captured files, so treat them as
secret-bearing until reviewed.
`run --download remote=local` copies successful-run artifacts back to the local
machine without adding file bytes to coordinator logs.
Test results are stored as structured summaries when `--junit` or
`results.junit` is configured.

`--timing-json` includes sync phases and command phases. Commands can add
user-defined phases by printing marker lines to stdout or stderr:

```sh
echo CRABBOX_PHASE:install
pnpm install --frozen-lockfile
echo CRABBOX_PHASE:build
pnpm build
echo CRABBOX_PHASE:test
pnpm test
```

When local `CRABBOX_ENV_ALLOW` is set, `run` prints the variable names selected
for forwarding plus safe metadata such as whether secret-looking names are set
and their value length. Values are never printed. Delegated Testbox providers
print that this forwarding is unsupported and that secrets belong in the
provider workflow.

## Remote Debugging

Use SSH for live process and filesystem inspection:

```sh
bin/crabbox ssh --id blue-lobster
bin/crabbox inspect --id blue-lobster --json
```

Useful remote checks:

```sh
crabbox-ready
test -f /var/lib/crabbox/bootstrapped
df -h
free -h
ps aux --sort=-%cpu | head
```

If a lease was created with `--keep`, SSH remains available until `crabbox stop`, idle expiry, or the TTL cap removes it.

For a concise pre-command capability snapshot, add `--preflight`:

```sh
bin/crabbox run --id blue-lobster --preflight -- pnpm test:changed
```

The preflight prints the remote user, remote cwd, sudo and apt availability,
Node, pnpm, Docker, and bubblewrap from the same command workdir. It sources the
Actions handoff env file when present, and marks the workspace as raw or
Actions-hydrated. Raw workspaces with Actions hydration configured print the
exact hydrate command suggestion and whether the selected provider/target
supports hydration.

## Actions Hydration

`crabbox actions hydrate` dispatches the configured workflow and waits for a ready marker. The workflow run URL and marker path are the key correlation points.

Use:

```sh
bin/crabbox actions hydrate --id blue-lobster
bin/crabbox inspect --id blue-lobster --json
```

The hydrated run writes non-secret handoff data for later `crabbox run --id blue-lobster` commands. Secrets and OIDC tokens remain workflow-step scoped unless the workflow intentionally writes its own short-lived handoff.

## Live Provider Debugging

For live provider or end-to-end test runs, prefer an Actions-hydrated lease
when tests need Node, pnpm, Docker services, repository secrets, or GitHub OIDC:

```sh
crabbox warmup --provider aws --class beast --keep
crabbox actions hydrate --id blue-lobster --workflow .github/workflows/hydrate.yml
mkdir -p .crabbox/logs
CRABBOX_ENV_ALLOW=OPENAI_API_KEY,OPENAI_BASE_URL \
  crabbox run --id blue-lobster \
  --preflight \
  --timing-json \
  --capture-stdout .crabbox/logs/live-provider.stdout.log \
  --capture-stderr .crabbox/logs/live-provider.stderr.log \
  --capture-on-fail \
  --shell 'echo CRABBOX_PHASE:install; pnpm install --frozen-lockfile; echo CRABBOX_PHASE:test; pnpm test:live'
```

For Blacksmith Testbox comparison runs, keep secrets in the Testbox workflow
environment. Crabbox will show that `CRABBOX_ENV_ALLOW` forwarding is
unsupported because Blacksmith owns command execution:

```sh
CRABBOX_ENV_ALLOW=OPENAI_API_KEY \
  crabbox run --provider blacksmith-testbox \
  --blacksmith-workflow .github/workflows/ci-check-testbox.yml \
  --blacksmith-job test \
  --preflight \
  -- pnpm test:live:providers
```

## Worker Logs

When the coordinator path fails before SSH, check Worker logs and Durable Object errors. The symptoms usually group into:

- auth failure;
- cost limit rejection;
- provider quota or capacity rejection;
- provider API failure;
- Durable Object alarm or state transition bug.

Keep the lease ID, owner, org, provider, class, and request time when comparing CLI output to Worker logs.

## Gaps

Current Crabbox observability is enough for maintainer operations, but not yet a full analytics product. Missing pieces:

- alerting on budget or failure-rate thresholds;
- dashboard UI.
