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

func waitForSSH(ctx context.Context, target SSHTarget, stderr io.Writer) error {
	deadline := time.Now().Add(12 * time.Minute)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			return exit(5, "timed out waiting for SSH on %s", target.Host)
		}
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(target.Host, target.Port), 5*time.Second)
		if err == nil {
			_ = conn.Close()
			if runSSHQuiet(ctx, target, "test -x /usr/local/bin/crabbox-ready && crabbox-ready >/tmp/crabbox-ready.log 2>&1") == nil {
				return nil
			}
		}
		fmt.Fprintf(stderr, "waiting for %s:%s bootstrap...\n", target.Host, target.Port)
		time.Sleep(10 * time.Second)
	}
}

func runSSHQuiet(ctx context.Context, target SSHTarget, remote string) error {
	cmd := exec.CommandContext(ctx, "ssh", sshArgs(target, remote)...)
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
		"-p", target.Port,
		target.User + "@" + target.Host,
		remote,
	}
}

func rsync(ctx context.Context, target SSHTarget, src, dst string, excludes []string, stdout, stderr io.Writer) error {
	args := []string{
		"-az", "--delete",
		"--stats",
		"-e", strings.Join([]string{
			"ssh",
			"-i", shellQuote(target.Key),
			"-o", "BatchMode=yes",
			"-o", "StrictHostKeyChecking=accept-new",
			"-p", shellQuote(target.Port),
		}, " "),
	}
	for _, exclude := range excludes {
		args = append(args, "--exclude", exclude)
	}
	args = append(args, ensureTrailingSlash(src), target.User+"@"+target.Host+":"+dst+"/")
	cmd := exec.CommandContext(ctx, "rsync", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
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

func remoteGitHydrate(workdir string) string {
	return "cd " + shellQuote(workdir) + " && " +
		"if git rev-parse --is-inside-work-tree >/dev/null 2>&1 && git remote get-url origin >/dev/null 2>&1; then " +
		"git fetch --quiet --unshallow origin main || git fetch --quiet --depth=1000 origin main || git fetch --quiet origin main || true; " +
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
