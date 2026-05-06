# cache

`crabbox cache` inspects, purges, or warms caches on a leased box.

```sh
crabbox cache stats --id blue-lobster
crabbox cache stats --id blue-lobster --json
crabbox cache warm --id blue-lobster -- pnpm install --frozen-lockfile
crabbox cache purge --id blue-lobster --kind pnpm --force
```

## Subcommands

```text
cache stats   show usage for each cache kind on the lease
cache warm    run a command in the synced workdir to populate caches
cache purge   delete one or all cache kinds (requires --force)
```

`--id` accepts the canonical `cbx_...` lease ID or an active friendly
slug. Cache commands SSH to the box, touch the lease, and validate the
local repo claim. Add `--reclaim` to move an existing claim from another
repo.

## Cache Kinds

```text
pnpm     /var/cache/crabbox/pnpm
npm      /var/cache/crabbox/npm
docker   Docker layer/image cache (host-managed)
git      /var/cache/crabbox/git (shared origin objects)
all      every kind enabled in repo config
```

Repo `cache.pnpm`, `cache.npm`, `cache.docker`, and `cache.git` toggles
control which kinds `stats` reports and which kinds `purge --kind all`
removes. Disabled kinds are omitted from stats, are not purged by
`--kind all`, and asking to purge a disabled specific kind fails early.

## stats

```sh
crabbox cache stats --id blue-lobster
```

Prints sizes for each enabled cache kind:

```text
pnpm    8.4GiB
npm     1.2GiB
docker  18.7GiB
git     430MiB
```

`--json` returns the same data as a structured object.

## warm

```sh
crabbox cache warm --id blue-lobster -- pnpm install --frozen-lockfile
crabbox cache warm --id blue-lobster -- docker compose pull
```

Runs a command in the synced repo workdir for that lease. On boxes
prepared by `crabbox actions hydrate`, it uses the hydrated
`$GITHUB_WORKSPACE` and sources the workflow env handoff, just like
`crabbox run` does.

Use warm for one-off cache priming when you do not want to record a full
run history entry.

## purge

```sh
crabbox cache purge --id blue-lobster --kind pnpm --force
crabbox cache purge --id blue-lobster --kind all --force
```

Removes the named cache kind from the lease. `--force` is required to
prevent accidental purges. If `cache.maxGB` is set, purge is rarely
needed - the runner trims the oldest entries automatically when caches
exceed the cap.

## Flags

```text
--id <lease-id-or-slug>     target lease (required)
--kind pnpm|npm|docker|git|all   for purge
--force                     required for purge
--reclaim                   move local claim from another repo
--json                      stats as JSON
```

## When To Use Cache

Caches are speed hints, not source of truth. The synced worktree remains
authoritative.

- Use `cache stats` to confirm a long-lived warm box is gaining benefit
  from cached packages.
- Use `cache warm` to prime a fresh lease before handing it to agents that
  run many short commands.
- Use `cache purge` when a corrupt cache is poisoning a build (rare;
  usually the underlying tool's own cache reset works first).

Disposable leases lose cache state when the VM is deleted; kept leases
can reuse cache state across repeated agent runs. For shared baked
images, see [Prebaked runner images](../features/prebaked-images.md).

Related docs:

- [Cache controls](../features/cache.md)
- [Performance](../performance.md)
- [run](run.md)
- [actions](actions.md)
