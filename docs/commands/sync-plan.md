# sync-plan

`crabbox sync-plan` prints the local sync manifest without leasing a box.
Use it to preview what `crabbox run` would send before paying for a cold
sync, or after editing `.crabboxignore` to confirm artifacts dropped out
of the manifest.

```sh
crabbox sync-plan
crabbox sync-plan --limit 10
crabbox sync-plan --limit 25 --json
```

## What It Reads

`sync-plan` uses the same Git file-list manifest, `.crabboxignore`, and
`sync.exclude` rules as `crabbox run`:

- tracked files from `git ls-files --cached`;
- nonignored untracked files from
  `git ls-files --others --exclude-standard`;
- root `.crabboxignore` patterns;
- repo-local `sync.exclude` patterns;
- Crabbox's default cache/build excludes.

It does not require a lease, does not call the broker, and does not call
any provider API.

## Output

Default output prints:

- candidate file count and total bytes;
- tracked deletes that would be applied remotely;
- the largest files;
- the largest first or second-level directories.

```text
files: 1843
bytes: 312.5MiB
tracked deletes: 0

largest files:
  84.5MiB  assets/demo.mp4
  12.4MiB  fixtures/sample-data.json
  ...

largest directories:
  140.2MiB  assets
   80.1MiB  fixtures
   ...
```

## Flags

```text
--limit <n>   show this many files and directories in each top list (default 5)
--json        print structured JSON output
```

`--limit 0` shows the full lists (use sparingly; large repos produce big
output).

## Use Cases

- preview a first sync before warming a beast-class lease;
- find sneaky directories that grew (`.cache/`, `dist/`, generated assets);
- audit `.crabboxignore` after adding new excludes;
- compare repo footprint over time as part of repo health checks.

The numbers `sync-plan` prints are upper bounds; rsync's actual transfer
size depends on what is already on the remote runner. Repeat sync after a
warmup is much smaller because the manifest matches the remote fingerprint
and rsync ships only changed bytes.

Related docs:

- [run](run.md)
- [Sync](../features/sync.md)
- [Configuration](../features/configuration.md)
