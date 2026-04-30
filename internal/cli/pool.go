package cli

import (
	"context"
	"encoding/json"
	"fmt"
)

func (a App) pool(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "list" {
		return exit(2, "usage: crabbox pool list [--json]")
	}
	fs := newFlagSet("pool list", a.Stderr)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args[1:]); err != nil {
		return exit(2, "%v", err)
	}
	client, err := newHetznerClient()
	if err != nil {
		return err
	}
	servers, err := client.ListCrabboxServers(ctx)
	if err != nil {
		return err
	}
	if *jsonOut {
		return json.NewEncoder(a.Stdout).Encode(servers)
	}
	for _, s := range servers {
		fmt.Fprintf(a.Stdout, "%-10d %-28s %-12s %-8s %-15s lease=%s keep=%s\n",
			s.ID, s.Name, s.Status, s.ServerType.Name, s.PublicNet.IPv4.IP, s.Labels["lease"], s.Labels["keep"])
	}
	return nil
}

func (a App) machine(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return exit(2, "usage: crabbox machine cleanup [--dry-run]")
	}
	switch args[0] {
	case "cleanup":
		return a.cleanup(ctx, args[1:])
	default:
		return exit(2, "unknown machine command %q", args[0])
	}
}

func (a App) cleanup(ctx context.Context, args []string) error {
	fs := newFlagSet("machine cleanup", a.Stderr)
	dryRun := fs.Bool("dry-run", false, "only print")
	if err := fs.Parse(args); err != nil {
		return exit(2, "%v", err)
	}
	client, err := newHetznerClient()
	if err != nil {
		return err
	}
	servers, err := client.ListCrabboxServers(ctx)
	if err != nil {
		return err
	}
	for _, s := range servers {
		if s.Labels["keep"] == "true" {
			continue
		}
		fmt.Fprintf(a.Stderr, "delete server id=%d name=%s\n", s.ID, s.Name)
		if !*dryRun {
			if err := client.DeleteServer(ctx, s.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
