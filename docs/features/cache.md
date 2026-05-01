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

Commands:

```sh
crabbox cache stats --id cbx_...
crabbox cache warm --id cbx_... -- pnpm install --frozen-lockfile
crabbox cache purge --id cbx_... --kind pnpm --force
```

Caches are speed hints, not source of truth. The synced worktree remains authoritative. Disposable leases lose cache state when the VM is deleted; kept leases can reuse cache state across repeated agent runs.
