package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
)

type App struct {
	Stdout io.Writer
	Stderr io.Writer
}

func Run(ctx context.Context, args []string) error {
	app := App{Stdout: os.Stdout, Stderr: os.Stderr}
	return app.Run(ctx, args)
}

func (a App) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		a.usage()
		return exit(2, "missing command")
	}

	switch args[0] {
	case "-h", "--help", "help":
		a.usage()
		return nil
	case "-v", "--version", "version":
		fmt.Fprintln(a.Stdout, version)
		return nil
	case "doctor":
		return a.doctor(ctx, args[1:])
	case "pool":
		return a.pool(ctx, args[1:])
	case "machine":
		return a.machine(ctx, args[1:])
	case "warmup":
		return a.warmup(ctx, args[1:])
	case "run":
		return a.runCommand(ctx, args[1:])
	case "stop", "release":
		return a.stop(ctx, args[1:])
	default:
		return exit(2, "unknown command %q", args[0])
	}
}

func (a App) usage() {
	fmt.Fprintln(a.Stdout, `crabbox leases remote Hetzner boxes, syncs a worktree, runs commands, and cleans up.

Usage:
  crabbox --version
  crabbox doctor
  crabbox warmup [--profile openclaw-check] [--class beast] [--keep]
  crabbox run [--profile openclaw-check] [--class beast] [--ttl 90m] [--keep] -- <command...>
  crabbox pool list
  crabbox machine cleanup [--dry-run]
  crabbox stop <lease-or-server-id>

Environment:
  HCLOUD_TOKEN or HETZNER_TOKEN
  CRABBOX_COORDINATOR, optional Cloudflare coordinator URL
  CRABBOX_COORDINATOR_TOKEN, optional coordinator bearer token
  CRABBOX_SSH_KEY, default ~/.ssh/id_ed25519
  CRABBOX_DEFAULT_CLASS, default beast`)
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}
