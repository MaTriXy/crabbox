# Repository Onboarding

Read when:

- changing `crabbox init`;
- adding repo-local config keys;
- changing generated workflow or agent instructions.

`crabbox init` prepares a repository for Crabbox without requiring users to remember the full command surface.

Generated files:

```text
.crabbox.yaml
.github/workflows/crabbox.yml
.agents/skills/crabbox/SKILL.md
```

Repo-local config should hold project-specific behavior:

- default profile;
- class;
- sync excludes;
- sync options;
- base ref for changed-test hydration;
- environment allowlist.

Generated agent instructions should point agents toward warmup/reuse flows and project-specific test commands. Generated workflow stubs are the bridge for Actions-backed hydration: Crabbox should dispatch or host real GitHub Actions jobs rather than reimplement Actions syntax locally.

Related docs:

- [init command](../commands/init.md)
- [Sync](sync.md)
- [CLI](../cli.md)
