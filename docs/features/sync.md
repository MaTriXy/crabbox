# Sync

Read when:

- changing rsync behavior;
- debugging missing or stale files on a runner;
- changing Git seeding, fingerprints, excludes, or env forwarding.

Crabbox syncs the current checkout to the leased runner before running a command.
It syncs the Git-managed working set, not the whole directory tree:

- tracked files from `git ls-files --cached`;
- nonignored untracked files from `git ls-files --others --exclude-standard`;
- root `.crabboxignore` patterns, repo-local `sync.exclude` patterns, and
  Crabbox's default cache/build excludes.

Ignored build output, dependency folders, `.git`, and common local caches stay out of the transfer. This keeps first syncs close to the code that CI would see while still letting agents test uncommitted edits.

Sync flow:

1. pick the local repository root;
2. seed remote Git from the configured origin/base ref when possible;
3. build a NUL-delimited sync manifest from Git's tracked and nonignored file list;
4. print a file/byte estimate and enforce configured large-sync guardrails;
5. compute or reuse a sync fingerprint from the commit, dirty metadata, and manifest;
6. skip rsync when the fingerprint matches;
7. feed the manifest to rsync with `--files-from=- --from0`;
8. delete previously managed remote files that disappeared from the manifest when delete sync is enabled;
9. run sanity checks for mass tracked deletions;
10. hydrate configured base-ref history for changed-test workflows.

The remote manifest deletion step only removes paths Crabbox previously synced. It does not delete workflow-created state, package caches, `.git`, or other local runner files outside the managed file list. Native Windows static targets use the same Git manifest but transfer it as a tar archive over OpenSSH instead of rsync.

In remote Git worktrees, Crabbox stores its sync metadata under `.git/crabbox` so repository status stays clean. Crabbox does not delete files under the worktree `.crabbox/` directory; that path remains available for repository-owned files and config.

Important controls:

```text
CRABBOX_SYNC_CHECKSUM
CRABBOX_SYNC_DELETE
CRABBOX_SYNC_GIT_SEED
CRABBOX_SYNC_FINGERPRINT
CRABBOX_SYNC_BASE_REF
CRABBOX_SYNC_TIMEOUT
CRABBOX_SYNC_WARN_FILES
CRABBOX_SYNC_WARN_BYTES
CRABBOX_SYNC_FAIL_FILES
CRABBOX_SYNC_FAIL_BYTES
CRABBOX_SYNC_ALLOW_LARGE
CRABBOX_ENV_ALLOW
```

Defaults:

```yaml
sync:
  timeout: 15m
  warnFiles: 50000
  warnBytes: 5368709120
  failFiles: 150000
  failBytes: 21474836480
  allowLarge: false
```

`crabbox run --force-sync-large` bypasses the fail thresholds for one run. `--debug` adds rsync progress/stat output; normal syncs still print a heartbeat when rsync is quiet for a while.

Use `crabbox sync-plan` to inspect the local manifest before leasing a box. It prints the candidate file count, total bytes, and the largest files/directories using the same excludes as `run`.

Repo-local config should hold project-specific excludes and env allowlists. Secrets must not be passed as command-line arguments or broad env globs.

Use `.crabboxignore` when you only need repo-local sync exclusions. The file is
read from the repository root. Blank lines and lines starting with `#` are
ignored; remaining lines are appended to `sync.exclude` and use the same matcher
as config excludes. Crabbox intentionally supports only `.crabboxignore`; there
is no short alias.

Related docs:

- [CLI](../cli.md)
- [run command](../commands/run.md)
- [sync-plan command](../commands/sync-plan.md)
- [Repository onboarding](repository-onboarding.md)
