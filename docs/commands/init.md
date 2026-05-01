# init

`crabbox init` onboards a repository for agent-first remote verification.

```sh
crabbox init
crabbox init --force
```

It writes:

- `.crabbox.json`
- `.github/workflows/crabbox.yml`
- `.agents/skills/crabbox/SKILL.md`

The generated workflow is intentionally conservative. It is a starting point for repo-specific hydration, not a full replacement for CI. Edit it to install dependencies, start service containers, and warm caches before agents begin repeated `crabbox run` calls.

Flags:

```text
--force                 overwrite generated files
--config <path>         repo config path
--workflow <path>       workflow path
--skill <path>          agent skill path
```
