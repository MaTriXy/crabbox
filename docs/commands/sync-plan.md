# sync-plan

`crabbox sync-plan` prints the local sync manifest without leasing a box.

```sh
crabbox sync-plan
crabbox sync-plan --limit 10
```

It uses the same Git file-list manifest, `.crabboxignore`, and config excludes
as `crabbox run`, then prints:

- candidate file count and total bytes;
- tracked deletes that would be applied remotely;
- largest files;
- largest first or second-level directories.

Use it before a cold sync when the preflight estimate looks too large, or after
editing `.crabboxignore` to confirm that local artifacts dropped out of the
manifest.

Related docs:

- [run](run.md)
- [Sync](../features/sync.md)
