# Cache Controls

Read when:

- changing warm-box cache behavior;
- debugging a slow repeated run;
- deciding whether to purge cached state.

Runner bootstrap prepares cache roots outside the synced source tree:

```text
/var/cache/crabbox/pnpm
/var/cache/crabbox/npm
/var/cache/crabbox/git
Docker local image/layer cache
```

Repo policy:

```yaml
cache:
  pnpm: true
  npm: true
  docker: true
  git: true
  maxGB: 80
  purgeOnRelease: false
```

The per-kind toggles control `cache stats` and `cache purge`. Disabled kinds are omitted from stats output and are not purged by `--kind all`; asking to purge a disabled specific kind fails early. Bootstrap may still create shared cache directories because they are harmless runner scaffolding.

Commands:

```sh
crabbox cache stats --id blue-lobster
crabbox cache warm --id blue-lobster -- pnpm install --frozen-lockfile
crabbox cache purge --id blue-lobster --kind pnpm --force
```

Caches are speed hints, not source of truth. The synced worktree remains authoritative. Disposable leases lose cache state when the VM is deleted; kept leases can reuse cache state across repeated agent runs.
