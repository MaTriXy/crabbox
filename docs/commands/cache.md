# cache

`crabbox cache` inspects, purges, or warms caches on a leased box.

```sh
crabbox cache stats --id blue-lobster
crabbox cache stats --id blue-lobster --json
crabbox cache warm --id blue-lobster -- pnpm install --frozen-lockfile
crabbox cache purge --id blue-lobster --kind pnpm --force
```

`--id` accepts the stable `cbx_...` ID or an active friendly slug. Cache commands that SSH to the box touch the lease and validate the local repo claim; add `--reclaim` to move an existing claim.

Cache kinds:

```text
pnpm
npm
docker
git
all
```

`cache warm` runs a command in the synced repo workdir for that lease. On boxes prepared by `crabbox actions hydrate`, it uses the hydrated `$GITHUB_WORKSPACE` and sources the workflow env handoff like `crabbox run`.

Repo `cache.pnpm`, `cache.npm`, `cache.docker`, and `cache.git` toggles control which kinds `stats` reports and which kinds `purge --kind all` removes.

Related docs:

- [Performance](../performance.md)
- [Cache controls](../features/cache.md)
