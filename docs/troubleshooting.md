# Troubleshooting

Read when:

- a lease fails to create;
- SSH never becomes ready;
- sync behaves unexpectedly;
- Actions hydration times out;
- docs deployment fails.

Start with:

```sh
bin/crabbox doctor
bin/crabbox config show
bin/crabbox status --json
bin/crabbox usage --scope all --json
```

## Broker Auth Fails

Symptoms:

- `401`;
- `403`;
- `missing broker token`;
- Cloudflare Access page instead of JSON.

Checks:

```sh
bin/crabbox config show
printenv CRABBOX_COORDINATOR
printenv CRABBOX_COORDINATOR_TOKEN
```

Fixes:

- configure the broker with `crabbox config set-broker`;
- ensure the CLI points at the Worker URL or the Access-protected route intentionally;
- ensure `CRABBOX_COORDINATOR_TOKEN` matches the Worker `CRABBOX_SHARED_TOKEN`.

## Lease Rejected By Cost Control

Symptoms:

- `cost_limit_exceeded`;
- lease request fails before provider creation.

Checks:

```sh
bin/crabbox usage --scope user --user "$(git config user.email)"
bin/crabbox usage --scope org --org openclaw
```

Fixes:

- raise the relevant monthly or active-lease limit;
- shorten `--idle-timeout`;
- choose a smaller `--class`;
- stop kept leases.

## Provider Capacity Or Quota Fails

Symptoms:

- class falls back from dedicated machines to smaller machines;
- AWS Spot request cannot be fulfilled;
- server create fails before SSH.

Checks:

```sh
bin/crabbox list --json
bin/crabbox usage --scope all
```

Fixes:

- choose a smaller class;
- set `CRABBOX_CAPACITY_REGIONS` for AWS Spot placement-score selection;
- set `CRABBOX_CAPACITY_STRATEGY=most-available`;
- raise Hetzner dedicated-core quota when dedicated classes are required;
- temporarily use AWS fallback capacity.

## SSH Never Becomes Ready

Symptoms:

- lease exists but `crabbox run` waits until SSH timeout;
- port `2222` is unreachable;
- `crabbox-ready` is missing.

Checks:

```sh
bin/crabbox inspect --id cbx_... --json
ssh -p 2222 crabbox@HOST test -f /var/lib/crabbox-ready
```

Fixes:

- wait for cloud-init to finish on fresh machines;
- verify security group or firewall allows port `2222`;
- inspect provider console output for cloud-init failures;
- retry the lease if bootstrap failed before creating the ready marker.

## Sync Looks Wrong

Symptoms:

- changed-test detection sees the wrong base;
- deleted files unexpectedly appear remotely;
- sync aborts on mass tracked deletions.
- sync warns or fails before rsync because the candidate tree is too large.

Checks:

```sh
git status --short
git ls-files --cached --others --exclude-standard | wc -l
bin/crabbox run --id cbx_... -- git status --short
bin/crabbox run --id cbx_... --sync-only --debug
```

Fixes:

- commit, stash, or intentionally keep local deletions before syncing;
- add generated directories to `.gitignore` or `.crabbox.yaml` `sync.exclude`;
- keep `.git`, build caches, and package caches out of the repo source tree;
- use `--force-sync-large` only after verifying the candidate file count and bytes are expected;
- check repo-local `.crabbox.yaml` sync excludes;
- rerun without relying on the sync fingerprint after large tree changes;
- verify base-ref hydration in repo config.

## Sync Stalls Or Times Out

Symptoms:

- rsync prints little output for a long time;
- `rsync timed out after ...`;
- a local cache directory made the first sync unexpectedly huge.

Checks:

```sh
bin/crabbox config show
bin/crabbox run --id cbx_... --sync-only --debug
```

Fixes:

- inspect the printed sync candidate estimate before retrying;
- lower `sync.timeout` for quick failure in agent loops, or raise it for intentionally large source transfers;
- tune `sync.warnFiles`, `sync.warnBytes`, `sync.failFiles`, and `sync.failBytes` in repo config;
- stop and warm a fresh lease if the remote workspace looks corrupted.

## Actions Hydration Times Out

Symptoms:

- `crabbox actions hydrate` dispatches a run but never sees the ready marker;
- later `crabbox run --id` does not enter the expected Actions workspace.

Checks:

```sh
bin/crabbox actions hydrate --id blue-lobster
bin/crabbox inspect --id blue-lobster --json
```

Fixes:

- open the workflow run URL and find the failed setup step;
- ensure the generated workflow writes the ready marker;
- confirm the workflow has permission to register or use the runner;
- keep secrets inside the workflow and only write non-secret handoff data.

## Docs Site Fails To Publish

Symptoms:

- Pages workflow fails during Pages setup;
- local docs build succeeds.

Checks:

```sh
node scripts/build-docs-site.mjs
gh run list --workflow pages.yml
```

Fixes:

- enable GitHub Pages for the repository or organization;
- rerun the Pages workflow after Pages is allowed;
- keep Markdown links relative so the static builder can rewrite them.
