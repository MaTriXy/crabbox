package cli

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

type GitHubRepo struct {
	Owner string
	Name  string
}

func (r GitHubRepo) Slug() string {
	if r.Owner == "" || r.Name == "" {
		return ""
	}
	return r.Owner + "/" + r.Name
}

func (a App) actions(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return exit(2, "usage: crabbox actions register|dispatch")
	}
	switch args[0] {
	case "register":
		return a.actionsRegister(ctx, args[1:])
	case "dispatch":
		return a.actionsDispatch(ctx, args[1:])
	default:
		return exit(2, "unknown actions command %q", args[0])
	}
}

func (a App) actionsRegister(ctx context.Context, args []string) error {
	fs := newFlagSet("actions register", a.Stderr)
	leaseIDFlag := fs.String("id", "", "existing lease id")
	repoFlag := fs.String("repo", "", "GitHub repository owner/name")
	nameFlag := fs.String("name", "", "runner name")
	labelsFlag := fs.String("labels", "", "comma-separated extra runner labels")
	versionFlag := fs.String("version", "", "actions/runner version or latest")
	ephemeralFlag := fs.Bool("ephemeral", true, "register runner as ephemeral")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *leaseIDFlag == "" {
		return exit(2, "actions register requires --id")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	repo, err := findRepo()
	if err != nil {
		return err
	}
	if *repoFlag != "" {
		cfg.Actions.Repo = *repoFlag
	}
	if *versionFlag != "" {
		cfg.Actions.RunnerVersion = *versionFlag
	}
	if flagWasSet(fs, "ephemeral") {
		cfg.Actions.Ephemeral = *ephemeralFlag
	}
	extraLabels := splitCommaList(*labelsFlag)
	ghRepo, err := resolveGitHubRepo(repo, cfg.Actions.Repo)
	if err != nil {
		return err
	}
	if coord, ok, err := newCoordinatorClient(cfg); err != nil {
		return err
	} else if ok {
		lease, err := coord.GetLease(ctx, *leaseIDFlag)
		if err != nil {
			return err
		}
		_, target, leaseID := leaseToServerTarget(lease, cfg)
		return a.registerGitHubActionsRunner(ctx, cfg, target, leaseID, ghRepo, *nameFlag, extraLabels)
	}
	_, target, leaseID, err := a.findLease(ctx, cfg, *leaseIDFlag)
	if err != nil {
		return err
	}
	return a.registerGitHubActionsRunner(ctx, cfg, target, leaseID, ghRepo, *nameFlag, extraLabels)
}

func (a App) actionsDispatch(ctx context.Context, args []string) error {
	fs := newFlagSet("actions dispatch", a.Stderr)
	repoFlag := fs.String("repo", "", "GitHub repository owner/name")
	workflowFlag := fs.String("workflow", "", "workflow file/name/id")
	refFlag := fs.String("ref", "", "workflow ref")
	fieldFlags := stringListFlag{}
	fs.Var(&fieldFlags, "f", "workflow input key=value")
	fs.Var(&fieldFlags, "field", "workflow input key=value")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	repo, err := findRepo()
	if err != nil {
		return err
	}
	if *repoFlag != "" {
		cfg.Actions.Repo = *repoFlag
	}
	if *workflowFlag != "" {
		cfg.Actions.Workflow = *workflowFlag
	}
	if *refFlag != "" {
		cfg.Actions.Ref = *refFlag
	}
	ghRepo, err := resolveGitHubRepo(repo, cfg.Actions.Repo)
	if err != nil {
		return err
	}
	if cfg.Actions.Workflow == "" {
		return exit(2, "actions dispatch requires --workflow or actions.workflow")
	}
	ref := cfg.Actions.Ref
	if ref == "" {
		ref = repo.BaseRef
	}
	if ref == "" {
		ref = "main"
	}
	cmdArgs := []string{"workflow", "run", cfg.Actions.Workflow, "--repo", ghRepo.Slug(), "--ref", ref}
	for _, field := range fieldFlags {
		if !strings.Contains(field, "=") {
			return exit(2, "workflow input must be key=value: %s", field)
		}
		cmdArgs = append(cmdArgs, "-f", field)
	}
	if err := runGH(ctx, repo.Root, cmdArgs...); err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "dispatched workflow=%s repo=%s ref=%s\n", cfg.Actions.Workflow, ghRepo.Slug(), ref)
	return nil
}

func (a App) registerGitHubActionsRunner(ctx context.Context, cfg Config, target SSHTarget, leaseID string, ghRepo GitHubRepo, nameOverride string, extraLabels []string) error {
	token, err := githubActionsRegistrationToken(ctx, ghRepo)
	if err != nil {
		return err
	}
	name := nameOverride
	if name == "" {
		name = "crabbox-" + leaseID
	}
	labels := githubActionsRunnerLabels(cfg, leaseID, extraLabels)
	script := githubActionsRunnerInstallScript(cfg.Actions.RunnerVersion, cfg.Actions.Ephemeral)
	remote := fmt.Sprintf("RUNNER_REPO=%s RUNNER_NAME=%s RUNNER_LABELS=%s RUNNER_TOKEN=%s bash -s", shellQuote(ghRepo.Slug()), shellQuote(name), shellQuote(strings.Join(labels, ",")), shellQuote(token))
	if err := runSSHInputQuiet(ctx, target, remote, script); err != nil {
		return exit(7, "register GitHub Actions runner on %s: %v", target.Host, err)
	}
	fmt.Fprintf(a.Stdout, "actions runner registered repo=%s name=%s labels=%s ephemeral=%t\n", ghRepo.Slug(), name, strings.Join(labels, ","), cfg.Actions.Ephemeral)
	return nil
}

func githubActionsRunnerLabels(cfg Config, leaseID string, extra []string) []string {
	labels := []string{
		"crabbox",
		"crabbox-" + sanitizeGitHubRunnerLabel(leaseID),
		"crabbox-profile-" + sanitizeGitHubRunnerLabel(cfg.Profile),
		"crabbox-class-" + sanitizeGitHubRunnerLabel(cfg.Class),
	}
	labels = append(labels, cfg.Actions.RunnerLabels...)
	labels = append(labels, extra...)
	return appendUniqueStrings(nil, labels...)
}

func githubActionsRunnerInstallScript(version string, ephemeral bool) string {
	if version == "" {
		version = "latest"
	}
	ephemeralArg := ""
	if ephemeral {
		ephemeralArg = "--ephemeral"
	}
	return fmt.Sprintf(`set -euo pipefail
if [ -z "${RUNNER_REPO:-}" ] || [ -z "${RUNNER_NAME:-}" ] || [ -z "${RUNNER_TOKEN:-}" ]; then
  echo "missing runner env" >&2
  exit 2
fi
version=%s
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) runner_arch=x64 ;;
  aarch64|arm64) runner_arch=arm64 ;;
  *) echo "unsupported runner arch: $arch" >&2; exit 2 ;;
esac
if [ "$version" = latest ]; then
  version="$(curl -fsSL https://api.github.com/repos/actions/runner/releases/latest | jq -r '.tag_name' | sed 's/^v//')"
fi
runner_dir="$HOME/actions-runner"
mkdir -p "$runner_dir"
cd "$runner_dir"
if [ ! -x ./config.sh ] || [ ! -f ".crabbox-runner-version-$version-$runner_arch" ]; then
  rm -rf ./*
  curl -fsSL -o actions-runner.tar.gz "https://github.com/actions/runner/releases/download/v${version}/actions-runner-linux-${runner_arch}-${version}.tar.gz"
  tar xzf actions-runner.tar.gz
  rm actions-runner.tar.gz
  touch ".crabbox-runner-version-$version-$runner_arch"
fi
if [ -f .runner ]; then
  ./config.sh remove --unattended --token "$RUNNER_TOKEN" || true
fi
sudo ./bin/installdependencies.sh >/tmp/crabbox-actions-runner-deps.log 2>&1 || true
./config.sh --unattended --replace %s --url "https://github.com/${RUNNER_REPO}" --token "$RUNNER_TOKEN" --name "$RUNNER_NAME" --labels "$RUNNER_LABELS"
cat >"$HOME/actions-runner/run-crabbox.sh" <<'RUNNER'
#!/usr/bin/env bash
set -euo pipefail
cd "$HOME/actions-runner"
exec ./run.sh
RUNNER
chmod +x "$HOME/actions-runner/run-crabbox.sh"
sudo tee /etc/systemd/system/crabbox-actions-runner.service >/dev/null <<SERVICE
[Unit]
Description=Crabbox GitHub Actions runner
After=network-online.target docker.service
Wants=network-online.target

[Service]
User=$(id -un)
WorkingDirectory=$HOME/actions-runner
ExecStart=$HOME/actions-runner/run-crabbox.sh
Restart=no

[Install]
WantedBy=multi-user.target
SERVICE
sudo systemctl daemon-reload
sudo systemctl enable --now crabbox-actions-runner.service
`, shellQuote(version), ephemeralArg)
}

func githubActionsRegistrationToken(ctx context.Context, repo GitHubRepo) (string, error) {
	out, err := ghOutput(ctx, "", "api", "-X", "POST", "repos/"+repo.Slug()+"/actions/runners/registration-token", "--jq", ".token")
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(out)
	if token == "" {
		return "", exit(3, "GitHub returned an empty runner registration token for %s", repo.Slug())
	}
	return token, nil
}

func resolveGitHubRepo(repo Repo, override string) (GitHubRepo, error) {
	if override != "" {
		return parseGitHubRepo(override)
	}
	return parseGitHubRepo(repo.RemoteURL)
}

var scpLikeGitHubRemote = regexp.MustCompile(`^[^@]+@github\.com:([^/]+)/(.+)$`)

func parseGitHubRepo(value string) (GitHubRepo, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return GitHubRepo{}, exit(2, "GitHub repo is unknown; set actions.repo or pass --repo owner/name")
	}
	if !strings.Contains(value, "://") {
		if match := scpLikeGitHubRemote.FindStringSubmatch(value); match != nil {
			return cleanGitHubRepo(match[1], match[2])
		}
		parts := strings.Split(strings.TrimSuffix(value, ".git"), "/")
		if len(parts) == 2 {
			return cleanGitHubRepo(parts[0], parts[1])
		}
	}
	u, err := url.Parse(value)
	if err == nil && strings.EqualFold(u.Host, "github.com") {
		parts := strings.Split(strings.Trim(path.Clean(u.Path), "/"), "/")
		if len(parts) >= 2 {
			return cleanGitHubRepo(parts[0], parts[1])
		}
	}
	return GitHubRepo{}, exit(2, "unsupported GitHub repo %q; expected owner/name or github.com remote", value)
}

func cleanGitHubRepo(owner, name string) (GitHubRepo, error) {
	owner = strings.TrimSpace(owner)
	name = strings.TrimSuffix(strings.TrimSpace(name), ".git")
	if owner == "" || name == "" {
		return GitHubRepo{}, exit(2, "invalid GitHub repo owner/name")
	}
	return GitHubRepo{Owner: owner, Name: name}, nil
}

func sanitizeGitHubRunnerLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "unknown"
	}
	return out
}

func ghOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", exit(3, "gh %s: %v\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func runGH(ctx context.Context, dir string, args ...string) error {
	_, err := ghOutput(ctx, dir, args...)
	return err
}

type stringListFlag []string

func (f *stringListFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *stringListFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}
