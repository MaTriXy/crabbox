package cli

import (
	"context"
	"encoding/json"
	"fmt"
)

func (a App) admin(ctx context.Context, args []string) error {
	if len(args) == 0 {
		a.printAdminHelp()
		return exit(2, "missing admin subcommand")
	}
	if wantsHelp(args) {
		a.printAdminHelp()
		return nil
	}
	switch args[0] {
	case "leases":
		return a.adminLeases(ctx, args[1:])
	case "release":
		return a.adminRelease(ctx, args[1:])
	case "delete":
		return a.adminDelete(ctx, args[1:])
	default:
		return exit(2, "unknown admin command %q", args[0])
	}
}

func (a App) printAdminHelp() {
	fmt.Fprintln(a.Stdout, `Usage:
  crabbox admin leases [flags]
  crabbox admin release <lease-id-or-slug> [flags]
  crabbox admin delete <lease-id-or-slug> --force [flags]

Subcommands:
  leases   List coordinator lease records
  release  Mark a lease released, optionally deleting the backing server
  delete   Delete the backing server and mark the lease released

Flags:
  leases:   --state <state> --owner <email> --org <name> --limit <n> --json
  release:  --id <lease-id-or-slug> --delete --json
  delete:   --id <lease-id-or-slug> --force --json

Docs:
  docs/commands/admin.md`)
}

func (a App) adminLeases(ctx context.Context, args []string) error {
	fs := newFlagSet("admin leases", a.Stderr)
	state := fs.String("state", "", "filter by state")
	owner := fs.String("owner", "", "filter by owner")
	org := fs.String("org", "", "filter by org")
	limit := fs.Int("limit", 100, "maximum leases")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	coord, err := configuredAdminCoordinator()
	if err != nil {
		return err
	}
	leases, err := coord.AdminLeases(ctx, *state, *owner, *org, *limit)
	if err != nil {
		return err
	}
	if *jsonOut {
		return json.NewEncoder(a.Stdout).Encode(leases)
	}
	for _, lease := range leases {
		fmt.Fprintf(a.Stdout, "%-16s %-16s %-8s %-10s %-14s %-24s owner=%s org=%s idle=%s expires=%s\n",
			lease.ID, blank(lease.Slug, "-"), lease.Provider, lease.State, lease.ServerType, lease.Host, lease.Owner, lease.Org, formatSecondsDuration(lease.IdleTimeoutSeconds), blank(lease.ExpiresAt, "-"))
	}
	return nil
}

func (a App) adminRelease(ctx context.Context, args []string) error {
	args, deleteAnywhere := extractBoolFlag(args, "delete")
	args, jsonAnywhere := extractBoolFlag(args, "json")
	fs := newFlagSet("admin release", a.Stderr)
	id := fs.String("id", "", "lease id or slug")
	deleteServer := fs.Bool("delete", false, "delete server while releasing")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *id == "" && fs.NArg() > 0 {
		*id = fs.Arg(0)
	}
	if *id == "" {
		return exit(2, "usage: crabbox admin release --id <lease-id-or-slug>")
	}
	if deleteAnywhere {
		*deleteServer = true
	}
	if jsonAnywhere {
		*jsonOut = true
	}
	coord, err := configuredAdminCoordinator()
	if err != nil {
		return err
	}
	lease, err := coord.AdminReleaseLease(ctx, *id, *deleteServer)
	if err != nil {
		return err
	}
	if *jsonOut {
		return json.NewEncoder(a.Stdout).Encode(lease)
	}
	fmt.Fprintf(a.Stdout, "released %s slug=%s state=%s delete=%t\n", lease.ID, blank(lease.Slug, "-"), lease.State, *deleteServer)
	return nil
}

func (a App) adminDelete(ctx context.Context, args []string) error {
	args, forceAnywhere := extractBoolFlag(args, "force")
	args, jsonAnywhere := extractBoolFlag(args, "json")
	fs := newFlagSet("admin delete", a.Stderr)
	id := fs.String("id", "", "lease id or slug")
	force := fs.Bool("force", false, "confirm deletion")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *id == "" && fs.NArg() > 0 {
		*id = fs.Arg(0)
	}
	if *id == "" {
		return exit(2, "usage: crabbox admin delete --id <lease-id-or-slug> --force")
	}
	if forceAnywhere {
		*force = true
	}
	if jsonAnywhere {
		*jsonOut = true
	}
	if !*force {
		return exit(2, "admin delete requires --force")
	}
	coord, err := configuredAdminCoordinator()
	if err != nil {
		return err
	}
	lease, err := coord.AdminDeleteLease(ctx, *id)
	if err != nil {
		return err
	}
	if *jsonOut {
		return json.NewEncoder(a.Stdout).Encode(lease)
	}
	fmt.Fprintf(a.Stdout, "deleted %s slug=%s state=%s\n", lease.ID, blank(lease.Slug, "-"), lease.State)
	return nil
}

func configuredCoordinator() (*CoordinatorClient, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	coord, ok, err := newCoordinatorClient(cfg)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, exit(2, "command requires a configured coordinator")
	}
	return coord, nil
}

func configuredAdminCoordinator() (*CoordinatorClient, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	if cfg.CoordAdminToken == "" {
		return nil, exit(2, "admin command requires broker.adminToken or CRABBOX_COORDINATOR_ADMIN_TOKEN")
	}
	cfg.CoordToken = cfg.CoordAdminToken
	coord, ok, err := newCoordinatorClient(cfg)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, exit(2, "admin command requires a configured coordinator")
	}
	return coord, nil
}
