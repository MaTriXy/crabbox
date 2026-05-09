# Semaphore

Read when:

- choosing `provider: semaphore`;
- debugging Semaphore host/project/auth config;
- changing Semaphore lease creation, SSH resolution, or cleanup.

`provider: semaphore` leases a Semaphore CI job and returns it to Crabbox as a
normal SSH target. Semaphore owns the CI job, project secrets, caches, machine
image, and API auth. Crabbox owns local config, slugs, repo claims, SSH sync,
remote command execution, timing summaries, and normalized list/status output.

## Auth

Prefer environment variables or user config. Do not commit Semaphore tokens in
repo config.

```sh
export CRABBOX_SEMAPHORE_HOST=myorg.semaphoreci.com
export CRABBOX_SEMAPHORE_PROJECT=my-app
export CRABBOX_SEMAPHORE_TOKEN=...
```

Fallback names are also accepted:

```sh
export SEMAPHORE_HOST=myorg.semaphoreci.com
export SEMAPHORE_PROJECT=my-app
export SEMAPHORE_API_TOKEN=...
```

Create an API token from `https://<host>/me/api-tokens`.

## Config

```yaml
provider: semaphore
target: linux
semaphore:
  host: myorg.semaphoreci.com
  project: my-app
  machine: f1-standard-2
  osImage: ubuntu2204
  idleTimeout: 30m
```

Equivalent one-off flags:

```sh
crabbox warmup --provider semaphore --semaphore-host myorg.semaphoreci.com --semaphore-project my-app
crabbox run --provider semaphore --semaphore-machine f1-standard-4 -- pnpm test
crabbox ssh --provider semaphore --id <slug>
crabbox status --provider semaphore --id <slug>
crabbox stop --provider semaphore <slug>
```

## Behavior

- `warmup` creates a standalone Semaphore job with a keepalive script and local
  Crabbox claim.
- `run` creates or reuses a job, syncs the current Git manifest over SSH, and
  runs the command through Crabbox's standard SSH executor.
- `ssh` prints the normal Crabbox SSH command for the resolved job.
- `status`, `list`, and `stop` operate on Semaphore jobs that Crabbox can map
  to local claims or provider metadata.
- `stop` posts the Semaphore job stop request and removes the local claim after
  provider cleanup succeeds.

## Boundaries

- Linux only.
- No Crabbox coordinator; Semaphore API auth is local/provider-native.
- No VNC, desktop, browser, code-server, or Actions hydration.
- `--class` maps poorly to Semaphore machines; prefer
  `--semaphore-machine` or `semaphore.machine`.
- `--type` is not used for Semaphore. Use the provider-specific machine field.
- `--checksum` works because Semaphore exposes a real SSH target.

## Troubleshooting

- `missing semaphore host`: set `CRABBOX_SEMAPHORE_HOST` or
  `semaphore.host`.
- `missing semaphore project`: set `CRABBOX_SEMAPHORE_PROJECT` or
  `semaphore.project`.
- `401 Unauthorized`: the token is wrong for the host, expired, or lacks access
  to the project.
- job reaches `RUNNING` but has no SSH endpoint: wait for Semaphore to attach
  debug SSH metadata, then retry `status` or `ssh`.
- invalid idle timeout fails before API calls; use Go duration syntax such as
  `30m`, `1h`, or `90m`.

Related docs:

- [Provider: Semaphore](../providers/semaphore.md)
- [Providers](providers.md)
- [Provider backends](../provider-backends.md)
