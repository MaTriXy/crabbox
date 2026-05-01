package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func (a App) initProject(_ context.Context, args []string) error {
	fs := newFlagSet("init", a.Stderr)
	force := fs.Bool("force", false, "overwrite generated files")
	workflow := fs.String("workflow", ".github/workflows/crabbox.yml", "workflow path")
	skill := fs.String("skill", ".agents/skills/crabbox/SKILL.md", "agent skill path")
	config := fs.String("config", ".crabbox.yaml", "repo config path")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	repo, err := findRepo()
	if err != nil {
		return err
	}
	files := map[string]string{
		filepath.Join(repo.Root, *config):   projectConfigTemplate(repo.Name),
		filepath.Join(repo.Root, *workflow): workflowTemplate(),
		filepath.Join(repo.Root, *skill):    skillTemplate(),
	}
	for path, content := range files {
		if err := writeInitFile(path, content, *force); err != nil {
			return err
		}
		fmt.Fprintf(a.Stdout, "wrote %s\n", path)
	}
	return nil
}

func writeInitFile(path, content string, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return exit(2, "%s already exists; use --force to overwrite", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return exit(2, "create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return exit(2, "write %s: %v", path, err)
	}
	return nil
}

func projectConfigTemplate(repoName string) string {
	return fmt.Sprintf(`profile: %s-check
class: beast
capacity:
  market: spot
  strategy: most-available
  fallback: on-demand-after-120s
actions:
  workflow: .github/workflows/crabbox.yml
  runnerLabels:
    - crabbox
  runnerVersion: latest
  ephemeral: true
sync:
  delete: true
  checksum: false
  gitSeed: true
  fingerprint: true
  exclude:
    - .cache
    - .turbo
    - dist
    - node_modules
env:
  allow:
    - CI
    - NODE_OPTIONS
ssh:
  user: crabbox
  port: "2222"
`, repoName)
}

func workflowTemplate() string {
	return `name: crabbox

on:
  workflow_dispatch:
    inputs:
      ref:
        description: "Git ref to hydrate"
        required: false
        type: string

permissions:
  contents: read

jobs:
  hydrate:
    runs-on: ubuntu-24.04
    timeout-minutes: 30
    steps:
      - uses: actions/checkout@v6
        with:
          ref: ${{ inputs.ref || github.ref }}
      - name: Hydrate
        run: |
          if [ -f package-lock.json ]; then npm ci; fi
          if [ -f pnpm-lock.yaml ]; then corepack enable && pnpm install --frozen-lockfile; fi
          if [ -f go.mod ]; then go mod download; fi
`
}

func skillTemplate() string {
	return `# Crabbox

Use Crabbox for remote Linux verification.

Workflow:
- Warm early: crabbox warmup --idle-timeout 90m
- Reuse the returned cbx_ id for all checks in the current task.
- Run checks with crabbox run --id <id> -- <command>.
- Use crabbox status --id <id> --wait before broad gates if needed.
- Use crabbox ssh --id <id> to inspect the runner when a failure needs live context.
- Stop with crabbox stop <id> when finished.

Do not debug product failures on a reused box that fails sync sanity. Stop it, warm a fresh box, and rerun.
`
}
