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
	provider := fs.String("provider", defaultConfig().Provider, "provider: hetzner or aws")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args[1:]); err != nil {
		return exit(2, "%v", err)
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.Provider = *provider
	if coord, ok, err := newCoordinatorClient(cfg); err != nil {
		return err
	} else if ok {
		machines, err := coord.Pool(ctx, cfg)
		if err != nil {
			return err
		}
		if *jsonOut {
			return json.NewEncoder(a.Stdout).Encode(machines)
		}
		for _, s := range machines {
			fmt.Fprintf(a.Stdout, "%-20s %-28s %-12s %-14s %-15s lease=%s keep=%s\n",
				s.ID, s.Name, s.Status, s.ServerType, s.Host, s.Labels["lease"], s.Labels["keep"])
		}
		return nil
	}
	if cfg.Provider == "aws" {
		client, err := newAWSClient(ctx, cfg)
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
			fmt.Fprintf(a.Stdout, "%-20s %-28s %-12s %-14s %-15s lease=%s keep=%s\n",
				s.DisplayID(), s.Name, s.Status, s.ServerType.Name, s.PublicNet.IPv4.IP, s.Labels["lease"], s.Labels["keep"])
		}
		return nil
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
		fmt.Fprintf(a.Stdout, "%-20s %-28s %-12s %-14s %-15s lease=%s keep=%s\n",
			s.DisplayID(), s.Name, s.Status, s.ServerType.Name, s.PublicNet.IPv4.IP, s.Labels["lease"], s.Labels["keep"])
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
	provider := fs.String("provider", defaultConfig().Provider, "provider: hetzner or aws")
	dryRun := fs.Bool("dry-run", false, "only print")
	if err := fs.Parse(args); err != nil {
		return exit(2, "%v", err)
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.Provider = *provider
	if cfg.Provider == "aws" {
		awsClient, err := newAWSClient(ctx, cfg)
		if err != nil {
			return err
		}
		servers, err := awsClient.ListCrabboxServers(ctx)
		if err != nil {
			return err
		}
		for _, s := range servers {
			if s.Labels["keep"] == "true" {
				continue
			}
			fmt.Fprintf(a.Stderr, "delete server id=%s name=%s\n", s.DisplayID(), s.Name)
			if !*dryRun {
				if err := awsClient.DeleteServer(ctx, s.CloudID); err != nil {
					return err
				}
			}
		}
		return nil
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
		fmt.Fprintf(a.Stderr, "delete server id=%s name=%s\n", s.DisplayID(), s.Name)
		if !*dryRun {
			if err := client.DeleteServer(ctx, s.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
