# cache

`crabbox cache` inspects, purges, or warms caches on a leased box.

```sh
crabbox cache stats --id cbx_...
crabbox cache stats --id cbx_... --json
crabbox cache warm --id cbx_... -- pnpm install --frozen-lockfile
crabbox cache purge --id cbx_... --kind pnpm --force
```

Cache kinds:

```text
pnpm
npm
docker
git
all
```

`cache warm` runs a command in the synced repo workdir for that lease. It is intended for kept or hydrated boxes where the repo already exists remotely.

Related docs:

- [Performance](../performance.md)
- [Cache controls](../features/cache.md)
