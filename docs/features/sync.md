# Sync

Read when:

- changing rsync behavior;
- debugging missing or stale files on a runner;
- changing Git seeding, fingerprints, excludes, or env forwarding.

Crabbox syncs the current dirty checkout to the leased runner before running a command.

Sync flow:

1. pick the local repository root;
2. seed remote Git from the configured origin/base ref when possible;
3. compute or reuse a sync fingerprint;
4. skip rsync when the fingerprint matches;
5. rsync local files into `/work/crabbox/<lease>/<repo>`;
6. apply delete semantics when configured;
7. run sanity checks for mass tracked deletions;
8. hydrate configured base-ref history for changed-test workflows.

Important controls:

```text
CRABBOX_SYNC_CHECKSUM
CRABBOX_SYNC_DELETE
CRABBOX_SYNC_GIT_SEED
CRABBOX_SYNC_FINGERPRINT
CRABBOX_SYNC_BASE_REF
CRABBOX_ENV_ALLOW
```

Repo-local config should hold project-specific excludes and env allowlists. Secrets must not be passed as command-line arguments or broad env globs.

Related docs:

- [CLI](../cli.md)
- [run command](../commands/run.md)
- [Repository onboarding](repository-onboarding.md)
