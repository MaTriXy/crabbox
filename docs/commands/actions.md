# actions

`crabbox actions` bridges a leased Crabbox machine into real GitHub Actions.

It does not parse workflow YAML locally. It uses GitHub's runner and workflow APIs:

- `actions register` gets a repository runner registration token through `gh api`, installs the official `actions/runner` package on an existing box, and starts it with systemd.
- `actions dispatch` calls `gh workflow run` for the configured workflow.

```sh
crabbox warmup --actions-runner --idle-timeout 90m
crabbox actions register --id cbx_123
crabbox actions dispatch -f testbox_id=cbx_123
```

Subcommands:

```text
register --id <lease> [--repo owner/name] [--name <runner-name>] [--labels <csv>] [--version latest] [--ephemeral=true]
dispatch [--repo owner/name] [--workflow <file|name|id>] [--ref <ref>] [-f key=value]
```

Config:

```yaml
actions:
  repo: owner/name
  workflow: .github/workflows/crabbox.yml
  ref: main
  runnerLabels:
    - crabbox
  runnerVersion: latest
  ephemeral: true
```

Workflow jobs should target the dynamic label printed by registration, for example `crabbox-cbx-123`, plus any static labels configured for the project.
