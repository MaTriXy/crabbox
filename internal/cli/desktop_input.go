package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

func (a App) desktopDoctor(ctx context.Context, args []string) error {
	target, cfg, leaseID, err := a.desktopCommandTarget(ctx, "desktop doctor", args, false)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "lease: %s provider=%s target=%s\n", leaseID, cfg.Provider, target.TargetOS)
	out, err := runSSHOutput(ctx, target, desktopDoctorRemoteCommand(target))
	if err != nil {
		return exit(5, "desktop doctor failed: %v", err)
	}
	fmt.Fprintln(a.Stdout, out)
	if isBlacksmithProvider(cfg.Provider) || isStaticProvider(cfg.Provider) {
		return nil
	}
	coord, useCoordinator, err := newTargetCoordinatorClient(cfg)
	if err == nil && useCoordinator && coord != nil && coord.Token != "" {
		rescueCtx := rescueContext{Cfg: cfg, Target: target, LeaseID: leaseID}
		status, err := coord.WebVNCStatus(ctx, leaseID)
		if err != nil {
			fmt.Fprintf(a.Stdout, "portal failed webvnc %v\n", err)
			printRescue(a.Stdout, rescueVNCBridgeDisconnected, err.Error(), webVNCStatusRescueCommand(rescueCtx), webVNCResetRescueCommand(rescueCtx))
		} else {
			fmt.Fprintf(a.Stdout, "portal ok webvnc bridge=%t viewer=%t\n", status.BridgeConnected, status.ViewerConnected)
			if status.ViewerConnected {
				printRescue(a.Stdout, rescueVNCStaleViewer, "close stale WebVNC tabs or reset this lease's WebVNC session", webVNCResetRescueCommand(rescueCtx))
			}
			if !status.BridgeConnected {
				printRescue(a.Stdout, rescueVNCBridgeNotRunning, "portal has no active WebVNC bridge for this lease", webVNCDaemonStartRescueCommand(rescueCtx), webVNCResetRescueCommand(rescueCtx))
			}
		}
	}
	return nil
}

func (a App) desktopClick(ctx context.Context, args []string) error {
	target, cfg, leaseID, err := a.desktopCommandTarget(ctx, "desktop click", args, true)
	if err != nil {
		return err
	}
	x, xOK := intFlagValue(args, "x")
	y, yOK := intFlagValue(args, "y")
	if !xOK || !yOK || x < 0 || y < 0 {
		return exit(2, "usage: crabbox desktop click --id <lease-id-or-slug> --x <n> --y <n>")
	}
	if out, err := runSSHCombinedOutput(ctx, target, desktopClickRemoteCommand(x, y)); err != nil {
		a.printDesktopInputRescue(classifyDesktopFailure(out), out, cfg, target, leaseID)
		return exit(5, "desktop click failed for %s: %v", leaseID, err)
	}
	fmt.Fprintf(a.Stdout, "clicked: lease=%s x=%d y=%d\n", leaseID, x, y)
	return nil
}

func (a App) desktopPaste(ctx context.Context, args []string) error {
	target, cfg, leaseID, err := a.desktopCommandTarget(ctx, "desktop paste", args, true)
	if err != nil {
		return err
	}
	text, err := desktopTextArgOrStdin(a.Stderr, args, "desktop paste")
	if err != nil {
		return err
	}
	var stdout, stderr strings.Builder
	if err := runSSHInput(ctx, target, desktopPasteRemoteCommand(), strings.NewReader(text), &stdout, &stderr); err != nil {
		a.printDesktopInputRescue(classifyDesktopFailure(stderr.String()+"\n"+stdout.String()), stderr.String()+"\n"+stdout.String(), cfg, target, leaseID)
		return exit(5, "desktop paste failed for %s: %v", leaseID, err)
	}
	fmt.Fprintf(a.Stdout, "pasted: lease=%s bytes=%d\n", leaseID, len(text))
	return nil
}

func (a App) desktopType(ctx context.Context, args []string) error {
	target, cfg, leaseID, err := a.desktopCommandTarget(ctx, "desktop type", args, true)
	if err != nil {
		return err
	}
	text, err := desktopTextArgOrStdin(a.Stderr, args, "desktop type")
	if err != nil {
		return err
	}
	if desktopShouldPasteForType(text) {
		var stdout, stderr strings.Builder
		if err := runSSHInput(ctx, target, desktopPasteRemoteCommand(), strings.NewReader(text), &stdout, &stderr); err != nil {
			a.printDesktopInputRescue(classifyDesktopFailure(stderr.String()+"\n"+stdout.String()), stderr.String()+"\n"+stdout.String(), cfg, target, leaseID)
			return exit(5, "desktop type paste fallback failed for %s: %v", leaseID, err)
		}
		fmt.Fprintf(a.Stdout, "typed: lease=%s method=paste bytes=%d\n", leaseID, len(text))
		return nil
	}
	if out, err := runSSHCombinedOutput(ctx, target, desktopTypeRemoteCommand(text)); err != nil {
		a.printDesktopInputRescue(classifyDesktopFailure(out), out, cfg, target, leaseID)
		return exit(5, "desktop type failed for %s: %v", leaseID, err)
	}
	fmt.Fprintf(a.Stdout, "typed: lease=%s method=xdotool bytes=%d\n", leaseID, len(text))
	return nil
}

func (a App) desktopKey(ctx context.Context, args []string) error {
	target, cfg, leaseID, err := a.desktopCommandTarget(ctx, "desktop key", args, true)
	if err != nil {
		return err
	}
	keys, err := desktopKeySequenceArg(args)
	if err != nil {
		return err
	}
	if strings.TrimSpace(keys) == "" {
		return exit(2, "usage: crabbox desktop key --id <lease-id-or-slug> <keys>")
	}
	if out, err := runSSHCombinedOutput(ctx, target, desktopKeyRemoteCommand(keys)); err != nil {
		a.printDesktopInputRescue(classifyDesktopFailure(out), out, cfg, target, leaseID)
		return exit(5, "desktop key failed for %s: %v", leaseID, err)
	}
	fmt.Fprintf(a.Stdout, "key: lease=%s keys=%s\n", leaseID, strings.TrimSpace(keys))
	return nil
}

func (a App) printDesktopInputRescue(problem, output string, cfg Config, target SSHTarget, leaseID string) {
	ctx := rescueContext{Cfg: cfg, Target: target, LeaseID: leaseID}
	printRescue(a.Stdout, problem, trimFailureDetail(output), desktopDoctorCommand(ctx))
}

func (a App) desktopCommandTarget(ctx context.Context, name string, args []string, requireLinux bool) (SSHTarget, Config, string, error) {
	defaults := defaultConfig()
	fs := newFlagSet(name, a.Stderr)
	provider := fs.String("provider", defaults.Provider, "provider: hetzner, aws, or ssh")
	id := fs.String("id", "", "lease id or slug")
	targetFlags := registerTargetFlags(fs, defaults)
	networkFlags := registerNetworkModeFlag(fs, defaults)
	if strings.HasSuffix(name, "click") {
		fs.Int("x", -1, "x coordinate")
		fs.Int("y", -1, "y coordinate")
	}
	if strings.HasSuffix(name, "paste") || strings.HasSuffix(name, "type") {
		fs.String("text", "", "text to enter")
	}
	if strings.HasSuffix(name, "key") {
		fs.String("keys", "", "xdotool key sequence")
	}
	if name == "artifacts video" {
		fs.String("output", "", "local MP4 output path")
		fs.Duration("duration", 10*time.Second, "video capture duration")
		fs.Float64("fps", 15, "video frames per second")
	}
	if err := parseFlags(fs, args); err != nil {
		return SSHTarget{}, Config{}, "", err
	}
	setIDFromFirstArg(fs, id)
	cfg, err := loadLeaseTargetConfig(fs, *provider, targetFlags, networkFlags, leaseTargetConfigOptions{Desktop: true})
	if err != nil {
		return SSHTarget{}, Config{}, "", err
	}
	if isBlacksmithProvider(cfg.Provider) {
		return SSHTarget{}, Config{}, "", exit(2, "desktop helpers are not supported for provider=%s; Blacksmith owns machine connectivity", cfg.Provider)
	}
	if err := requireLeaseID(*id, "crabbox "+name+" --id <lease-id-or-slug>", cfg); err != nil {
		return SSHTarget{}, Config{}, "", err
	}
	server, target, leaseID, err := a.resolveNetworkLeaseTarget(ctx, cfg, *id, false)
	if err != nil {
		return SSHTarget{}, Config{}, "", err
	}
	if err := enforceManagedLeaseCapabilities(cfg, server, leaseID); err != nil {
		return SSHTarget{}, Config{}, "", err
	}
	if requireLinux && target.TargetOS != targetLinux {
		return SSHTarget{}, Config{}, "", exit(2, "desktop input helpers currently require target=linux with xdotool")
	}
	a.touchLeaseTargetBestEffort(ctx, cfg, LeaseTarget{Server: server, SSH: target, LeaseID: leaseID}, "")
	return target, cfg, leaseID, nil
}

func desktopKeySequenceArg(args []string) (string, error) {
	defaults := defaultConfig()
	fs := newFlagSet("desktop key", io.Discard)
	fs.String("provider", defaults.Provider, "provider: hetzner, aws, or ssh")
	id := fs.String("id", "", "lease id or slug")
	registerTargetFlags(fs, defaults)
	registerNetworkModeFlag(fs, defaults)
	keys := fs.String("keys", "", "xdotool key sequence")
	if err := parseFlags(fs, args); err != nil {
		return "", err
	}
	if strings.TrimSpace(*keys) != "" {
		return *keys, nil
	}
	remaining := fs.Args()
	if *id == "" && len(remaining) > 0 {
		remaining = remaining[1:]
	}
	if len(remaining) == 0 {
		return "", nil
	}
	return remaining[0], nil
}

func desktopTextArgOrStdin(stderr io.Writer, args []string, name string) (string, error) {
	_ = stderr
	if text, ok := stringFlagValue(args, "text"); ok {
		return text, nil
	}
	info, err := os.Stdin.Stat()
	if err == nil && info.Mode()&os.ModeCharDevice != 0 {
		return "", exit(2, "usage: crabbox %s --id <lease-id-or-slug> --text <text>", name)
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", exit(2, "read stdin: %v", err)
	}
	return string(data), nil
}

func stringFlagValue(args []string, name string) (string, bool) {
	prefixes := []string{"--" + name + "=", "-" + name + "="}
	names := map[string]bool{"--" + name: true, "-" + name: true}
	for i, arg := range args {
		for _, prefix := range prefixes {
			if strings.HasPrefix(arg, prefix) {
				return strings.TrimPrefix(arg, prefix), true
			}
		}
		if names[arg] && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

func intFlagValue(args []string, name string) (int, bool) {
	value, ok := stringFlagValue(args, name)
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(value)
	return n, err == nil
}

func floatFlagValue(args []string, name string, fallback float64) float64 {
	value, ok := stringFlagValue(args, name)
	if !ok {
		return fallback
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return n
}

func durationFlagValue(args []string, name string, fallback time.Duration) time.Duration {
	value, ok := stringFlagValue(args, name)
	if !ok {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return duration
}

func desktopShouldPasteForType(text string) bool {
	if text == "" {
		return false
	}
	if strings.ContainsAny(text, "\n\r\t @+:/\\'\"`$&|;<>[]{}()!*?=") {
		return true
	}
	if len(text) > 64 {
		return true
	}
	return false
}

func desktopClickRemoteCommand(x, y int) string {
	return fmt.Sprintf(`set -eu
export DISPLAY="${DISPLAY:-:99}"
command -v xdotool >/dev/null 2>&1 || { echo "missing xdotool; warm a new --desktop lease or install xdotool" >&2; exit 127; }
xdotool mousemove %d %d click 1`, x, y)
}

func desktopKeyRemoteCommand(keys string) string {
	return `set -eu
export DISPLAY="${DISPLAY:-:99}"
command -v xdotool >/dev/null 2>&1 || { echo "missing xdotool; warm a new --desktop lease or install xdotool" >&2; exit 127; }
xdotool key --clearmodifiers ` + shellQuote(strings.TrimSpace(keys))
}

func desktopTypeRemoteCommand(text string) string {
	return `set -eu
export DISPLAY="${DISPLAY:-:99}"
command -v xdotool >/dev/null 2>&1 || { echo "missing xdotool; warm a new --desktop lease or install xdotool" >&2; exit 127; }
xdotool type --clearmodifiers --delay 1 -- ` + shellQuote(text)
}

func desktopPasteRemoteCommand() string {
	return `set -eu
export DISPLAY="${DISPLAY:-:99}"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
cat > "$tmp"
if command -v xclip >/dev/null 2>&1; then
  timeout 5s xclip -selection clipboard -loops 1 "$tmp" &
  clip_pid=$!
elif command -v xsel >/dev/null 2>&1; then
  timeout 5s xsel --clipboard --input < "$tmp" &
  clip_pid=$!
elif command -v wl-copy >/dev/null 2>&1; then
  wl-copy --paste-once < "$tmp" &
  clip_pid=$!
else
  echo "missing clipboard tool; warm a new --desktop lease or install xclip/xsel" >&2
  exit 127
fi
command -v xdotool >/dev/null 2>&1 || { echo "missing xdotool; warm a new --desktop lease or install xdotool" >&2; exit 127; }
sleep 0.2
xdotool key --clearmodifiers ctrl+v
wait "$clip_pid" || true`
}

func desktopDoctorRemoteCommand(target SSHTarget) string {
	if target.TargetOS != targetLinux {
		return `echo "session warn target unsupported repair=desktop doctor has full checks for linux/xvfb leases"`
	}
	return `set +e
export DISPLAY="${DISPLAY:-:99}"
check() {
  layer="$1"; item="$2"; shift 2
  if "$@" >/dev/null 2>&1; then
    echo "$layer ok $item"
  else
    echo "$layer failed $item repair=$CRABBOX_REPAIR"
  fi
}
CRABBOX_REPAIR="ensure DISPLAY=:99 is exported"; [ -n "$DISPLAY" ] && echo "session ok DISPLAY=$DISPLAY" || echo "session failed DISPLAY repair=export DISPLAY=:99"
CRABBOX_REPAIR="restart crabbox-xvfb.service"; check session xvfb pgrep -f "Xvfb :99"
CRABBOX_REPAIR="restart crabbox-desktop.service"; check session xfwm4 pgrep -x xfwm4
CRABBOX_REPAIR="restart crabbox-desktop.service"; check session panel pgrep -x xfce4-panel
CRABBOX_REPAIR="restart crabbox-x11vnc.service"; check vm vnc ss -ltn sport = :5900
CRABBOX_REPAIR="warm a new --desktop lease or install xdotool"; check input xdotool command -v xdotool
CRABBOX_REPAIR="warm a new --desktop lease or install xclip"; if command -v xclip >/dev/null 2>&1 || command -v xsel >/dev/null 2>&1 || command -v wl-copy >/dev/null 2>&1; then echo "input ok clipboard"; else echo "input failed clipboard repair=$CRABBOX_REPAIR"; fi
CRABBOX_REPAIR="warm with --browser or install Chrome/Chromium"; if [ -f /var/lib/crabbox/browser.env ]; then . /var/lib/crabbox/browser.env; fi; if [ -n "${BROWSER:-}" ] && [ -x "$BROWSER" ]; then echo "session ok browser=$BROWSER"; elif command -v google-chrome >/dev/null 2>&1 || command -v chromium >/dev/null 2>&1 || command -v chromium-browser >/dev/null 2>&1; then echo "session ok browser"; else echo "session failed browser repair=$CRABBOX_REPAIR"; fi
CRABBOX_REPAIR="warm a new --desktop lease or install ffmpeg"; check capture ffmpeg command -v ffmpeg
CRABBOX_REPAIR="restart crabbox-xvfb.service"; if command -v xrandr >/dev/null 2>&1; then size="$(xrandr 2>/dev/null | awk '/ connected/{getline; print $1; exit}')"; [ -n "$size" ] && echo "session ok screen=$size" || echo "session failed screen repair=$CRABBOX_REPAIR"; else echo "session failed screen repair=install x11-xserver-utils"; fi
CRABBOX_REPAIR="restart desktop services or install scrot"; if command -v scrot >/dev/null 2>&1; then tmp="$(mktemp --suffix=.png)" && scrot -z -o "$tmp" >/dev/null 2>&1 && test -s "$tmp"; ok=$?; rm -f "$tmp"; [ "$ok" -eq 0 ] && echo "capture ok screenshot" || echo "capture failed screenshot repair=$CRABBOX_REPAIR"; else echo "capture failed screenshot repair=$CRABBOX_REPAIR"; fi`
}
