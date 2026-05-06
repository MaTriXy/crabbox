# Environment Forwarding

Read when:

- adding a new env var that the remote command needs to see;
- debugging "why is `$CI` empty inside `crabbox run`?";
- writing a repo config that lets agents set tunable values without flags;
- reviewing a PR that loosens or tightens the env allowlist.

By default, `crabbox run` does not forward arbitrary local environment
variables to the remote command. Forwarding is opt-in and name-based: the
repo declares which variable names are allowed, and Crabbox forwards only
those that are present locally.

## Why Allowlist

Agents and CI environments run with rich and sometimes sensitive
environments: tokens, private credentials, terminal paths, vendor-specific
debug flags. Forwarding everything would:

- leak secrets to remote runners;
- introduce non-determinism between local and CI runs;
- make it impossible to reason about what affects a remote command.

Allowlist forwarding makes the contract explicit. The repo decides what
"counts" as input to the remote command, and the user can audit the
allowlist in `crabbox.yaml`.

## Configuration

```yaml
env:
  allow:
    - CI
    - NODE_OPTIONS
    - PROJECT_*
```

Rules:

- entries are env var names, not values;
- a trailing `*` is a prefix wildcard (`PROJECT_*` matches `PROJECT_FOO`,
  `PROJECT_BAR`);
- inline wildcards (`PROJECT_*_DEBUG`) are not supported;
- match is exact and case-sensitive;
- empty entries are ignored.

The user-side override is `CRABBOX_ENV_ALLOW`, a comma-separated list:

```sh
CRABBOX_ENV_ALLOW='CI,NODE_OPTIONS,PROJECT_*' crabbox run -- pnpm test
```

`CRABBOX_ENV_ALLOW` replaces the repo allowlist for that command rather than
appending to it. Use it for one-off tests; persistent allowances belong in
`env.allow`.

## What Gets Forwarded

For each env var in the allowlist, Crabbox checks whether the variable is
set locally. If it is, the variable is forwarded to the remote command with
the same name and value. If it is not set locally, nothing is forwarded -
Crabbox does not invent values.

The remote command sees the variables as part of its environment when run
through SSH:

```sh
ssh runner 'CI=true NODE_OPTIONS=--max_old_space_size=4096 cd workdir && pnpm test'
```

Quoting and escaping happen automatically. Values that contain shell
metacharacters are passed through safely.

## Capability-Injected Env

A small set of env vars is injected by Crabbox itself when the matching
capability is requested. These bypass the allowlist because Crabbox owns
them:

```text
DISPLAY=:99               when --desktop
CRABBOX_DESKTOP=1         when --desktop
BROWSER=<path>            when --browser, after probe
CHROME_BIN=<path>         when --browser, after probe
CRABBOX_BROWSER=1         when --browser
```

User-allowed env vars override capability-injected ones if they overlap.
Repos that need a different `BROWSER` value can include `BROWSER` in
`env.allow` and set it locally.

## Secrets

Do not put secrets in `env.allow` even if forwarding seems convenient.
Secrets belong in:

- the broker environment (Cloudflare Worker secrets) for provider
  credentials;
- the operator's credential store (`op`, AWS Vault, etc.) for short-lived
  tokens;
- per-runner image bake when the secret should be on every lease;
- post-bootstrap secret injection in repo-owned setup scripts (devcontainer,
  mise, repo-controlled `bin/setup`).

Crabbox forwards values it sees locally. If a secret leaks into the
allowlist, every run of every contributor will leak it.

## Examples

```yaml
env:
  allow:
    - CI                    # mark a remote command as CI-driven
    - NODE_OPTIONS          # adjust Node memory in test suites
    - PYTEST_ADDOPTS        # tune pytest flags from the local env
    - PROJECT_*             # repo's own debug knobs
    - VITEST_*              # let agents override vitest config
    - DEBUG                 # `debug` package selector
```

Common things you usually do not allow:

```text
HOME, USER, PATH, SHELL    runner already has its own
SSH_*                       leaks SSH agent state
GITHUB_TOKEN                use Actions hydration or runner setup
AWS_*                       use IAM roles or instance profile
*_API_KEY, *_TOKEN          use a secret manager
```

## Inspecting Forwarding

`crabbox run --debug` prints the set of env vars that were forwarded for
that invocation. Use it to verify that the allowlist matches expectations
before debugging "why does the remote command not see this variable?".

```sh
$ crabbox run --debug -- env | grep '^PROJECT'
[crabbox] forwarding env: CI NODE_OPTIONS PROJECT_FOO PROJECT_BAR
PROJECT_FOO=value
PROJECT_BAR=other-value
```

Variables that match the allowlist but are unset locally are not in the
forwarded list, so the debug line is the source of truth for "what did the
remote command actually see".

Related docs:

- [Sync](sync.md)
- [Configuration](configuration.md)
- [run command](../commands/run.md)
- [Capabilities](capabilities.md)
- [Security](../security.md)
