package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	failureTailLines        = 40
	failureTailLineBytes    = 16 * 1024
	phaseMarkerPendingBytes = 4 * 1024
)

type streamTailBuffer struct {
	max     int
	lines   []string
	pending string
}

func newStreamTailBuffer(max int) *streamTailBuffer {
	if max <= 0 {
		max = failureTailLines
	}
	return &streamTailBuffer{max: max}
}

func (b *streamTailBuffer) Write(p []byte) (int, error) {
	text := b.pending + string(p)
	parts := strings.SplitAfter(text, "\n")
	b.pending = ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		if strings.HasSuffix(part, "\n") {
			b.append(truncateFailureTailLine(strings.TrimRight(part, "\r\n")))
			continue
		}
		b.pending = truncateFailureTailLine(part)
	}
	return len(p), nil
}

func (b *streamTailBuffer) Lines() []string {
	lines := append([]string(nil), b.lines...)
	if b.pending != "" {
		lines = append(lines, b.pending)
	}
	if len(lines) > b.max {
		lines = lines[len(lines)-b.max:]
	}
	return lines
}

func (b *streamTailBuffer) append(line string) {
	b.lines = append(b.lines, line)
	if len(b.lines) > b.max {
		b.lines = b.lines[len(b.lines)-b.max:]
	}
}

func truncateFailureTailLine(line string) string {
	if len(line) <= failureTailLineBytes {
		return line
	}
	return "[truncated] " + line[len(line)-failureTailLineBytes:]
}

type commandPhaseTracker struct {
	mu           sync.Mutex
	current      string
	currentStart time.Time
	phases       []timingPhase
}

type CommandPhaseTracker = commandPhaseTracker

func newCommandPhaseTracker(start time.Time) *commandPhaseTracker {
	return &commandPhaseTracker{current: "user-command", currentStart: start}
}

func NewCommandPhaseTracker(start time.Time) *CommandPhaseTracker {
	return newCommandPhaseTracker(start)
}

func FinishCommandPhaseTracker(tracker *CommandPhaseTracker, at time.Time) []TimingPhase {
	if tracker == nil {
		return nil
	}
	return tracker.Finish(at)
}

func (t *commandPhaseTracker) StartPhase(name string, at time.Time) {
	name = sanitizePhaseName(name)
	if name == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.finishCurrentLocked(at)
	t.current = name
	t.currentStart = at
}

func (t *commandPhaseTracker) Finish(at time.Time) []timingPhase {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.finishCurrentLocked(at)
	out := make([]timingPhase, len(t.phases))
	copy(out, t.phases)
	return out
}

func (t *commandPhaseTracker) finishCurrentLocked(at time.Time) {
	if t.current == "" || t.currentStart.IsZero() {
		return
	}
	if at.Before(t.currentStart) {
		at = t.currentStart
	}
	t.phases = append(t.phases, timingPhase{Name: t.current, Ms: at.Sub(t.currentStart).Milliseconds()})
	t.current = ""
	t.currentStart = time.Time{}
}

type phaseMarkerWriter struct {
	tracker *commandPhaseTracker
	pending string
}

type PhaseMarkerWriter = phaseMarkerWriter

func NewPhaseMarkerWriter(tracker *CommandPhaseTracker) *PhaseMarkerWriter {
	return &phaseMarkerWriter{tracker: tracker}
}

func (w *phaseMarkerWriter) Write(p []byte) (int, error) {
	if w == nil || w.tracker == nil {
		return len(p), nil
	}
	text := w.pending + string(p)
	for {
		i := strings.IndexByte(text, '\n')
		if i < 0 {
			break
		}
		w.observeLine(text[:i])
		text = text[i+1:]
	}
	w.pending = truncatePhaseMarkerPending(text)
	return len(p), nil
}

func (w *phaseMarkerWriter) Flush() {
	if w != nil && w.pending != "" {
		w.observeLine(w.pending)
		w.pending = ""
	}
}

func (w *phaseMarkerWriter) observeLine(line string) {
	if name, ok := phaseNameFromLine(line); ok {
		w.tracker.StartPhase(name, time.Now())
	}
}

func truncatePhaseMarkerPending(line string) string {
	if len(line) <= phaseMarkerPendingBytes {
		return line
	}
	return line[len(line)-phaseMarkerPendingBytes:]
}

func phaseNameFromLine(line string) (string, bool) {
	const prefix = "CRABBOX_PHASE:"
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}
	name := sanitizePhaseName(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
	return name, name != ""
}

func sanitizePhaseName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) > 80 {
		name = name[:80]
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-' || r == '.':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return b.String()
}

func formatCommandPhaseTimings(phases []timingPhase) string {
	parts := make([]string, 0, len(phases))
	for _, phase := range phases {
		if phase.Name == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%s", phase.Name, time.Duration(phase.Ms)*time.Millisecond))
	}
	return strings.Join(parts, ",")
}

func maybePrintEnvForwardingSummary(w io.Writer, provider, behavior string, allow []string, env map[string]string) {
	if strings.TrimSpace(os.Getenv("CRABBOX_ENV_ALLOW")) == "" {
		return
	}
	printEnvForwardingSummary(w, provider, behavior, allow, env)
}

func printEnvForwardingSummary(w io.Writer, provider, behavior string, allow []string, env map[string]string) {
	if w == nil {
		return
	}
	names := make([]string, 0, len(env))
	for name := range env {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := make([]string, 0, len(names))
	for _, name := range names {
		entries = append(entries, envMetadata(name, env[name]))
	}
	if len(entries) == 0 {
		fmt.Fprintf(w, "env forwarding provider=%s behavior=%s matched=none allow=%s\n", provider, behavior, strings.Join(allow, ","))
		return
	}
	fmt.Fprintf(w, "env forwarding provider=%s behavior=%s vars=%s\n", provider, behavior, strings.Join(entries, ","))
}

func PrintEnvForwardingSummary(w io.Writer, provider, behavior string, allow []string, env map[string]string) {
	printEnvForwardingSummary(w, provider, behavior, allow, env)
}

func envMetadata(name, value string) string {
	state := "set"
	if value == "" {
		state = "empty"
	}
	if envNameLooksSecret(name) {
		return fmt.Sprintf("%s=%s len=%d secret=true", name, state, len(value))
	}
	return fmt.Sprintf("%s=%s", name, state)
}

func envNameLooksSecret(name string) bool {
	upper := strings.ToUpper(name)
	for _, marker := range []string{"KEY", "TOKEN", "SECRET", "PASSWORD", "PASS", "CREDENTIAL", "AUTH"} {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return false
}

func printRunContextSummary(w io.Writer, coord *CoordinatorClient, cfg Config, server Server, target SSHTarget, leaseID, workdir string, hydrated bool, actionsURL string, recorder *runRecorder) {
	if w == nil {
		return
	}
	runID := ""
	if recorder != nil {
		runID = recorder.runID
	}
	workspace := "raw"
	if hydrated {
		workspace = "actions-hydrated"
	}
	fmt.Fprintln(w, "run context:")
	fmt.Fprintf(w, "  run=%s portal=%s logs=%s\n", blank(runID, "-"), runPortalURL(coord, runID), runLogsURL(coord, runID))
	fmt.Fprintf(w, "  lease=%s slug=%s provider=%s target=%s type=%s\n", leaseID, blank(serverSlug(server), "-"), cfg.Provider, blank(target.TargetOS, cfg.TargetOS), server.ServerType.Name)
	fmt.Fprintf(w, "  ssh=%s@%s:%s ip=%s\n", redactedSSHUser(cfg, server, target), target.Host, target.Port, blank(server.PublicNet.IPv4.IP, target.Host))
	fmt.Fprintf(w, "  workdir=%s workspace=%s actions=%s\n", workdir, workspace, blank(actionsURL, "-"))
}

func runPortalURL(coord *CoordinatorClient, runID string) string {
	if coord == nil || coord.BaseURL == "" || runID == "" {
		return "-"
	}
	return strings.TrimRight(coord.BaseURL, "/") + "/portal/runs/" + url.PathEscape(runID)
}

func runLogsURL(coord *CoordinatorClient, runID string) string {
	if coord == nil || coord.BaseURL == "" || runID == "" {
		return "-"
	}
	return strings.TrimRight(coord.BaseURL, "/") + "/v1/runs/" + url.PathEscape(runID) + "/logs"
}

func printRemoteCapabilityPreflight(ctx context.Context, w io.Writer, cfg Config, target SSHTarget, leaseID, workdir, actionsEnvFile string, hydrated bool, actionsURL string, hydrateSupported bool, env map[string]string) {
	if w == nil {
		return
	}
	for _, line := range remotePreflightWorkspaceLines(cfg, target, leaseID, workdir, hydrated, actionsURL, hydrateSupported) {
		fmt.Fprintln(w, line)
	}
	if isWindowsNativeTarget(target) {
		fmt.Fprintln(w, "remote preflight skipped: native Windows capability probe is not implemented")
		return
	}
	out, err := runSSHCombinedOutput(ctx, target, remoteCapabilityPreflightCommand(workdir, env, actionsEnvFile))
	if err != nil {
		fmt.Fprintf(w, "remote preflight failed: %v\n", err)
		if strings.TrimSpace(out) != "" {
			fmt.Fprintf(w, "remote preflight output: %s\n", strings.TrimSpace(out))
		}
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) != "" {
			fmt.Fprintf(w, "remote preflight %s\n", strings.TrimSpace(line))
		}
	}
}

func printDelegatedPreflightUnsupported(w io.Writer, provider string) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, "remote preflight provider=%s delegated unsupported; provider owns workspace and command transport\n", provider)
}

func remotePreflightWorkspaceLines(cfg Config, target SSHTarget, leaseID, workdir string, hydrated bool, actionsURL string, hydrateSupported bool) []string {
	workspace := "raw"
	if hydrated {
		workspace = "actions-hydrated"
	}
	lines := []string{fmt.Sprintf("remote preflight workspace=%s workdir=%s hydrate_supported=%t", workspace, workdir, hydrateSupported)}
	if actionsURL != "" {
		lines = append(lines, "remote preflight actions="+actionsURL)
	}
	if !hydrated && strings.TrimSpace(cfg.Actions.Workflow) != "" {
		lines = append(lines, "remote preflight hydrate_suggestion="+hydrateCommandSuggestion(cfg, target, leaseID, hydrateSupported))
	}
	return lines
}

func hydrateCommandSuggestion(cfg Config, target SSHTarget, leaseID string, supported bool) string {
	args := []string{"crabbox", "actions", "hydrate", "--id", leaseID}
	if cfg.Provider != "" {
		args = append(args, "--provider", cfg.Provider)
	}
	targetOS := firstNonBlank(target.TargetOS, cfg.TargetOS)
	if targetOS != "" {
		args = append(args, "--target", targetOS)
	}
	windowsMode := firstNonBlank(target.WindowsMode, cfg.WindowsMode)
	if targetOS == targetWindows && windowsMode != "" {
		args = append(args, "--windows-mode", windowsMode)
	}
	if cfg.Actions.Workflow != "" {
		args = append(args, "--workflow", cfg.Actions.Workflow)
	}
	if cfg.Actions.Job != "" {
		args = append(args, "--job", cfg.Actions.Job)
	}
	command := strings.Join(readableShellWords(args), " ")
	if !supported {
		command += " (unsupported for this provider/target)"
	}
	return command
}

func remoteCapabilityPreflightCommand(workdir string, env map[string]string, envFile string) string {
	script := `printf 'user=%s\n' "$(id -un 2>/dev/null || whoami 2>/dev/null || printf unknown)"
printf 'cwd=%s\n' "$(pwd -P 2>/dev/null || pwd)"
if command -v sudo >/dev/null 2>&1; then
  if sudo -n true >/dev/null 2>&1; then printf 'sudo=yes\n'; else printf 'sudo=no-password-required-failed\n'; fi
else
  printf 'sudo=missing\n'
fi
if command -v apt-get >/dev/null 2>&1; then printf 'apt=yes\n'; else printf 'apt=missing\n'; fi
if command -v node >/dev/null 2>&1; then printf 'node=%s\n' "$(node --version 2>/dev/null || printf present)"; else printf 'node=missing\n'; fi
if command -v pnpm >/dev/null 2>&1; then printf 'pnpm=%s\n' "$(pnpm --version 2>/dev/null || printf present)"; else printf 'pnpm=missing\n'; fi
if command -v docker >/dev/null 2>&1; then printf 'docker=%s\n' "$(docker --version 2>/dev/null | sed 's/,.*//')"; else printf 'docker=missing\n'; fi
if command -v bwrap >/dev/null 2>&1; then printf 'bubblewrap=yes\n'; else printf 'bubblewrap=missing\n'; fi`
	return remoteShellCommandWithEnvFile(workdir, env, envFile, script)
}

func captureFailureArtifacts(ctx context.Context, target SSHTarget, workdir, leaseID, runID string) (local string, bytes int, err error) {
	if isWindowsNativeTarget(target) {
		return "", 0, exit(2, "capture-on-fail is not supported for native Windows targets")
	}
	name := safeCaptureName(firstNonBlank(runID, leaseID, "run")) + "-" + time.Now().UTC().Format("20060102T150405Z") + ".tar.gz"
	remotePath := ".crabbox/" + name
	if out, err := runSSHCombinedOutput(ctx, target, remoteFailureCaptureCommand(workdir, remotePath)); err != nil {
		return "", 0, exit(7, "capture-on-fail prepare: %v: %s", err, strings.TrimSpace(out))
	}
	defer func() {
		if out, cleanupErr := runSSHCombinedOutput(ctx, target, remoteRemoveFailureCaptureCommand(workdir, remotePath)); cleanupErr != nil && err == nil {
			err = exit(7, "capture-on-fail remote cleanup: %v: %s", cleanupErr, strings.TrimSpace(out))
		}
	}()
	localPath := filepath.Join(".crabbox", "captures", name)
	bytes, local, err = downloadRemoteFile(ctx, target, workdir, remotePath+"="+localPath)
	return local, bytes, err
}

func safeCaptureName(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "run"
	}
	return b.String()
}

func remoteFailureCaptureCommand(workdir, remotePath string) string {
	var script bytes.Buffer
	script.WriteString("set -eu\n")
	script.WriteString("cd " + shellQuote(workdir) + "\n")
	script.WriteString("mkdir -p .crabbox\n")
	script.WriteString("out=" + shellQuote(remotePath) + "\n")
	script.WriteString(`manifest=.crabbox/capture-manifest.txt
files=.crabbox/capture-files.txt
{
  printf 'captured_at=%s\n' "$(date -Is 2>/dev/null || date)"
  printf 'host=%s\n' "$(hostname 2>/dev/null || printf unknown)"
  printf 'pwd=%s\n' "$(pwd -P 2>/dev/null || pwd)"
  printf 'note=%s\n' 'local-only failure capture; caller owns redaction before sharing'
} > "$manifest"
: > "$files"
for path in test-results playwright-report coverage junit.xml results.xml .crabbox/capture-manifest.txt; do
  if [ -e "$path" ]; then printf '%s\n' "$path" >> "$files"; fi
done
find . -maxdepth 3 -type f \( -name '*.log' -o -name 'junit*.xml' -o -name 'TEST-*.xml' \) \
  ! -path './test-results/*' \
  ! -path './playwright-report/*' \
  ! -path './coverage/*' \
  -print 2>/dev/null | sed 's#^\./##' >> "$files" || true
sort -u "$files" > "$files.sorted"
tar -czf "$out" -T "$files.sorted" 2>/dev/null || tar -czf "$out" "$manifest"
printf '%s\n' "$out"
`)
	return "bash -lc " + shellQuote(script.String())
}

func remoteRemoveFailureCaptureCommand(workdir, remotePath string) string {
	script := "set -eu\ncd " + shellQuote(workdir) + "\nrm -f -- " + shellQuote(remotePath)
	return "bash -lc " + shellQuote(script)
}

func printFailureTail(w io.Writer, label string, tail *streamTailBuffer, capturedPath string) {
	if w == nil {
		return
	}
	if capturedPath != "" {
		fmt.Fprintf(w, "%s tail: captured at %s\n", label, capturedPath)
		return
	}
	lines := tail.Lines()
	if len(lines) == 0 {
		fmt.Fprintf(w, "%s tail: empty\n", label)
		return
	}
	fmt.Fprintf(w, "%s tail last %d lines:\n", label, len(lines))
	for _, line := range lines {
		fmt.Fprintf(w, "%s\n", line)
	}
}
