package cli

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type SSHTarget struct {
	User string
	Host string
	Key  string
	Port string
}

func waitForSSH(ctx context.Context, target *SSHTarget, stderr io.Writer) error {
	return waitForSSHReady(ctx, target, stderr, "bootstrap", 20*time.Minute)
}

func waitForSSHReady(ctx context.Context, target *SSHTarget, stderr io.Writer, phase string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			return exit(5, "timed out waiting for SSH on %s during %s", target.Host, phase)
		}
		reachablePort := ""
		for _, port := range sshPortCandidates(target.Port) {
			probe := *target
			probe.Port = port
			conn, err := net.DialTimeout("tcp", net.JoinHostPort(probe.Host, probe.Port), 5*time.Second)
			if err != nil {
				continue
			}
			_ = conn.Close()
			if reachablePort == "" {
				reachablePort = probe.Port
			}
			if runSSHQuiet(ctx, probe, "test -x /usr/local/bin/crabbox-ready && crabbox-ready >/tmp/crabbox-ready.log 2>&1") == nil {
				if target.Port != probe.Port {
					fmt.Fprintf(stderr, "using ssh port %s for %s (configured %s not ready)\n", probe.Port, target.Host, target.Port)
					target.Port = probe.Port
				}
				return nil
			}
		}
		if reachablePort != "" {
			fmt.Fprintf(stderr, "waiting for %s:%s %s toolchain...\n", target.Host, reachablePort, phase)
		} else {
			fmt.Fprintf(stderr, "waiting for %s:%s %s...\n", target.Host, target.Port, phase)
		}
		time.Sleep(10 * time.Second)
	}
}

func sshPortCandidates(port string) []string {
	if port == "" || port == "22" {
		return []string{"22"}
	}
	return []string{port, "22"}
}

func runSSHQuiet(ctx context.Context, target SSHTarget, remote string) error {
	cmd := exec.CommandContext(ctx, "ssh", sshArgs(target, remote)...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func runSSHOutput(ctx context.Context, target SSHTarget, remote string) (string, error) {
	cmd := exec.CommandContext(ctx, "ssh", sshArgs(target, remote)...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func runSSHInputQuiet(ctx context.Context, target SSHTarget, remote, input string) error {
	cmd := exec.CommandContext(ctx, "ssh", sshArgs(target, remote)...)
	cmd.Stdin = strings.NewReader(input)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func runSSHStream(ctx context.Context, target SSHTarget, remote string, stdout, stderr io.Writer) int {
	cmd := exec.CommandContext(ctx, "ssh", sshArgs(target, remote)...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if ok := asExitError(err, &exitErr); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}
	return 7
}

func sshArgs(target SSHTarget, remote string) []string {
	return []string{
		"-i", target.Key,
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=" + filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"),
		"-o", "ConnectTimeout=10",
		"-o", "ConnectionAttempts=3",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=2",
		"-p", target.Port,
		target.User + "@" + target.Host,
		remote,
	}
}

type rsyncOptions struct {
	Debug    bool
	Delete   bool
	Checksum bool
}

func rsync(ctx context.Context, target SSHTarget, src, dst string, excludes []string, stdout, stderr io.Writer, opts rsyncOptions) error {
	args := []string{
		"-az",
		"-e", strings.Join([]string{
			"ssh",
			"-i", shellQuote(target.Key),
			"-o", "BatchMode=yes",
			"-o", "StrictHostKeyChecking=accept-new",
			"-o", "UserKnownHostsFile=" + shellQuote(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts")),
			"-o", "ConnectTimeout=10",
			"-o", "ConnectionAttempts=3",
			"-o", "ServerAliveInterval=15",
			"-o", "ServerAliveCountMax=2",
			"-p", shellQuote(target.Port),
		}, " "),
	}
	if opts.Delete {
		args = append(args, "--delete")
	}
	if opts.Checksum {
		args = append(args, "--checksum")
	}
	for _, exclude := range excludes {
		args = append(args, "--exclude", exclude)
	}
	if opts.Debug {
		args = append(args, "--stats", "--itemize-changes")
	}
	args = append(args, ensureTrailingSlash(src), target.User+"@"+target.Host+":"+dst+"/")
	start := time.Now()
	cmd := exec.CommandContext(ctx, "rsync", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if opts.Debug {
		fmt.Fprintf(stderr, "rsync elapsed=%s checksum=%t delete=%t\n", time.Since(start).Round(time.Millisecond), opts.Checksum, opts.Delete)
	}
	return err
}

func ensureTrailingSlash(path string) string {
	if strings.HasSuffix(path, "/") {
		return path
	}
	return path + "/"
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func remoteCommand(workdir string, env map[string]string, command []string) string {
	var b strings.Builder
	b.WriteString("cd ")
	b.WriteString(shellQuote(workdir))
	b.WriteString(" && ")
	for k, v := range env {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(shellQuote(v))
		b.WriteByte(' ')
	}
	b.WriteString("bash -lc ")
	b.WriteString(shellQuote(`exec "$@"`))
	b.WriteString(" bash")
	for _, word := range command {
		b.WriteByte(' ')
		b.WriteString(shellQuote(word))
	}
	return b.String()
}

func remoteShellCommand(workdir string, env map[string]string, script string) string {
	var b strings.Builder
	b.WriteString("cd ")
	b.WriteString(shellQuote(workdir))
	b.WriteString(" && ")
	for k, v := range env {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(shellQuote(v))
		b.WriteByte(' ')
	}
	b.WriteString("bash -lc ")
	b.WriteString(shellQuote(script))
	return b.String()
}

func shellWords(words []string) []string {
	out := make([]string, 0, len(words))
	for _, w := range words {
		out = append(out, shellQuote(w))
	}
	return out
}

func remoteMkdir(workdir string) string {
	return "mkdir -p " + shellQuote(workdir)
}

func remoteGitHydrate(workdir, baseRef string) string {
	if baseRef == "" {
		return "true"
	}
	return "cd " + shellQuote(workdir) + " && " +
		"if git rev-parse --is-inside-work-tree >/dev/null 2>&1 && git remote get-url origin >/dev/null 2>&1; then " +
		"git fetch --quiet --unshallow origin " + shellQuote(baseRef) + " || git fetch --quiet --depth=1000 origin " + shellQuote(baseRef) + " || git fetch --quiet origin " + shellQuote(baseRef) + " || true; " +
		"fi"
}

func remoteGitSeed(workdir, remoteURL, head string) string {
	remoteURL = normalizeGitRemoteURL(remoteURL)
	if remoteURL == "" || head == "" {
		return "true"
	}
	parent := filepath.ToSlash(filepath.Dir(workdir))
	return "if [ ! -d " + shellQuote(workdir+"/.git") + " ]; then " +
		"mkdir -p " + shellQuote(parent) + "; " +
		"tmp=$(mktemp -d " + shellQuote(parent+"/.seed.XXXXXX") + "); " +
		"if git clone --quiet --filter=blob:none --no-checkout " + shellQuote(remoteURL) + " \"$tmp\" >/dev/null 2>&1; then " +
		"(cd \"$tmp\" && (git fetch --quiet --depth=1 origin " + shellQuote(head) + " || true) && (git checkout --quiet " + shellQuote(head) + " || git checkout --quiet FETCH_HEAD || true)); " +
		"rm -rf " + shellQuote(workdir) + " && mv \"$tmp\" " + shellQuote(workdir) + "; " +
		"else rm -rf \"$tmp\"; fi; " +
		"fi"
}

func normalizeGitRemoteURL(remoteURL string) string {
	if strings.HasPrefix(remoteURL, "git@github.com:") {
		return "https://github.com/" + strings.TrimSuffix(strings.TrimPrefix(remoteURL, "git@github.com:"), ".git") + ".git"
	}
	return remoteURL
}

func remoteReadSyncFingerprint(workdir string) string {
	return "cat " + shellQuote(workdir+"/.crabbox/sync-fingerprint") + " 2>/dev/null || true"
}

func remoteWriteSyncFingerprint(workdir, fingerprint string) string {
	return "mkdir -p " + shellQuote(workdir+"/.crabbox") + " && printf %s " + shellQuote(fingerprint) + " > " + shellQuote(workdir+"/.crabbox/sync-fingerprint")
}

func remoteSyncSanity(workdir string, allowMassDeletions bool) string {
	allowValue := ""
	if allowMassDeletions {
		allowValue = "1"
	}
	return "cd " + shellQuote(workdir) + " && " +
		"if test -d .git && git status --short >/tmp/crabbox-git-status 2>/dev/null; then " +
		"deletions=$(awk '/^ D|^D / { n++ } END { print n+0 }' /tmp/crabbox-git-status); " +
		"if [ " + shellQuote(allowValue) + " != '1' ] && [ \"$deletions\" -ge 200 ]; then " +
		"echo \"remote sync sanity failed: $deletions tracked deletions\" >&2; exit 66; " +
		"fi; " +
		"fi"
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if asExitError(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}
	return 1
}

func parseServerID(s string) (int64, bool) {
	id, err := strconv.ParseInt(s, 10, 64)
	return id, err == nil
}
