# Actions Hydration

Read when:

- wiring Crabbox into an existing GitHub Actions CI setup;
- changing `crabbox actions hydrate`;
- deciding whether setup belongs in Crabbox or in a repository workflow.

Actions hydration lets a repository reuse its existing GitHub Actions setup without asking Crabbox to understand workflow YAML.

The flow:

1. `crabbox warmup --idle-timeout 90m` leases a machine.
2. `crabbox actions hydrate --id cbx_...` registers that machine as an ephemeral self-hosted runner for the repository.
3. Crabbox dispatches the configured workflow with the lease ID, dynamic runner label, and keepalive timeout.
4. The workflow runs on `[self-hosted, crabbox-cbx-...]`, checks out the repo, installs dependencies, starts services, warms caches, and performs any repo-specific setup.
5. The workflow writes `$HOME/.crabbox/actions/<lease>.env` with `WORKSPACE`, `RUN_ID`, and `READY_AT`.
6. `crabbox run --id cbx_... -- <command>` reads that marker and syncs the local dirty checkout into `$GITHUB_WORKSPACE`.

The important boundary: project setup lives in the repository workflow. Crabbox owns runner registration, dispatch, marker waiting, SSH sync, and command execution. It does not contain repository-specific setup code.

Repo config:

```yaml
actions:
  workflow: .github/workflows/crabbox.yml
  ref: main
  runnerLabels:
    - crabbox
  runnerVersion: latest
  ephemeral: true
```

Hydrate workflows must accept:

```yaml
on:
  workflow_dispatch:
    inputs:
      crabbox_id:
        required: true
        type: string
      crabbox_runner_label:
        required: true
        type: string
      crabbox_keep_alive_minutes:
        required: false
        default: "90"
        type: string
```

The job should run on the dynamic label:

```yaml
runs-on: [self-hosted, "${{ inputs.crabbox_runner_label }}"]
```

The workflow marks readiness after setup:

```sh
mkdir -p "$HOME/.crabbox/actions"
state="$HOME/.crabbox/actions/${{ inputs.crabbox_id }}.env"
tmp="${state}.tmp"
{
  echo "WORKSPACE=${GITHUB_WORKSPACE}"
  echo "RUN_ID=${GITHUB_RUN_ID}"
  echo "READY_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} > "$tmp"
mv "$tmp" "$state"
```

The final workflow step should keep the job alive while agents run commands. It can exit when `$HOME/.crabbox/actions/${{ inputs.crabbox_id }}.stop` appears or when its timeout expires.

Related docs:

- [actions command](../commands/actions.md)
- [run command](../commands/run.md)
- [warmup command](../commands/warmup.md)
- [Repository onboarding](repository-onboarding.md)
