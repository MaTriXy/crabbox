package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

type cacheEntry struct {
	Kind  string `json:"kind"`
	Path  string `json:"path,omitempty"`
	Bytes int64  `json:"bytes,omitempty"`
	Note  string `json:"note,omitempty"`
}

func (a App) cache(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return exit(2, "usage: crabbox cache list|stats|purge|warm")
	}
	switch args[0] {
	case "list", "stats":
		return a.cacheStats(ctx, args[1:])
	case "purge":
		return a.cachePurge(ctx, args[1:])
	case "warm":
		return a.cacheWarm(ctx, args[1:])
	default:
		return exit(2, "unknown cache command %q", args[0])
	}
}

func (a App) cacheStats(ctx context.Context, args []string) error {
	fs := newFlagSet("cache stats", a.Stderr)
	id := fs.String("id", "", "lease id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *id == "" && fs.NArg() > 0 {
		*id = fs.Arg(0)
	}
	if *id == "" {
		return exit(2, "usage: crabbox cache stats --id <lease-id>")
	}
	target, _, err := a.cacheTarget(ctx, *id)
	if err != nil {
		return err
	}
	out, err := runSSHOutput(ctx, target, remoteCacheStats())
	if err != nil {
		return err
	}
	entries := parseCacheStats(out)
	if *jsonOut {
		return json.NewEncoder(a.Stdout).Encode(entries)
	}
	for _, entry := range entries {
		if entry.Note != "" {
			fmt.Fprintf(a.Stdout, "%-8s %s\n", entry.Kind, entry.Note)
			continue
		}
		fmt.Fprintf(a.Stdout, "%-8s %-32s %s\n", entry.Kind, formatBytes(entry.Bytes), entry.Path)
	}
	return nil
}

func (a App) cachePurge(ctx context.Context, args []string) error {
	fs := newFlagSet("cache purge", a.Stderr)
	id := fs.String("id", "", "lease id")
	kind := fs.String("kind", "all", "cache kind: pnpm, npm, docker, git, or all")
	force := fs.Bool("force", false, "confirm purge")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *id == "" && fs.NArg() > 0 {
		*id = fs.Arg(0)
	}
	if *id == "" {
		return exit(2, "usage: crabbox cache purge --id <lease-id> --kind <kind> --force")
	}
	if !*force {
		return exit(2, "cache purge requires --force")
	}
	target, _, err := a.cacheTarget(ctx, *id)
	if err != nil {
		return err
	}
	if err := runSSHQuiet(ctx, target, remoteCachePurge(*kind)); err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "purged cache kind=%s lease=%s\n", *kind, *id)
	return nil
}

func (a App) cacheWarm(ctx context.Context, args []string) error {
	fs := newFlagSet("cache warm", a.Stderr)
	id := fs.String("id", "", "lease id")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	command := fs.Args()
	if len(command) > 0 && command[0] == "--" {
		command = command[1:]
	}
	if *id == "" {
		return exit(2, "cache warm requires --id")
	}
	if len(command) == 0 {
		return exit(2, "usage: crabbox cache warm --id <lease-id> -- <command...>")
	}
	target, cfg, err := a.cacheTarget(ctx, *id)
	if err != nil {
		return err
	}
	repo, err := findRepo()
	if err != nil {
		return err
	}
	workdir := filepath.ToSlash(filepath.Join(cfg.WorkRoot, *id, repo.Name))
	code := runSSHStream(ctx, target, remoteCommand(workdir, allowedEnv(cfg.EnvAllow), command), a.Stdout, a.Stderr)
	if code != 0 {
		return ExitError{Code: code, Message: fmt.Sprintf("cache warm command exited %d", code)}
	}
	return nil
}

func (a App) cacheTarget(ctx context.Context, id string) (SSHTarget, Config, error) {
	cfg, err := loadConfig()
	if err != nil {
		return SSHTarget{}, Config{}, err
	}
	_, target, _, err := a.resolveLeaseTarget(ctx, cfg, id)
	return target, cfg, err
}

func remoteCacheStats() string {
	return "for item in pnpm:/var/cache/crabbox/pnpm npm:/var/cache/crabbox/npm git:/var/cache/crabbox/git; do kind=${item%%:*}; path=${item#*:}; if [ -e \"$path\" ]; then bytes=$(du -sk \"$path\" 2>/dev/null | awk '{print $1*1024}'); printf '%s\\t%s\\t%s\\n' \"$kind\" \"$path\" \"${bytes:-0}\"; fi; done; if command -v docker >/dev/null 2>&1; then printf 'docker\\t\\t%s\\n' \"$(docker system df --format '{{.Type}}={{.Size}}' 2>/dev/null | paste -sd ',' -)\"; fi"
}

func remoteCachePurge(kind string) string {
	switch kind {
	case "pnpm":
		return "rm -rf /var/cache/crabbox/pnpm/*"
	case "npm":
		return "rm -rf /var/cache/crabbox/npm/*"
	case "git":
		return "rm -rf /var/cache/crabbox/git/*"
	case "docker":
		return "docker system prune -af >/dev/null 2>&1 || true"
	case "all":
		return "rm -rf /var/cache/crabbox/pnpm/* /var/cache/crabbox/npm/* /var/cache/crabbox/git/*; docker system prune -af >/dev/null 2>&1 || true"
	default:
		return "false"
	}
}

func parseCacheStats(output string) []cacheEntry {
	var entries []cacheEntry
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		entry := cacheEntry{Kind: parts[0], Path: parts[1]}
		if parts[1] == "" {
			entry.Note = parts[2]
		} else {
			fmt.Sscanf(parts[2], "%d", &entry.Bytes)
		}
		entries = append(entries, entry)
	}
	return entries
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
