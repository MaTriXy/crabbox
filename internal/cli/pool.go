package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (a App) pool(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "list" {
		return exit(2, "usage: crabbox pool list [--json]")
	}
	return a.list(ctx, args[1:])
}

func (a App) list(ctx context.Context, args []string) error {
	fs := newFlagSet("list", a.Stderr)
	provider := fs.String("provider", defaultConfig().Provider, "provider: hetzner or aws")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return err
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
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.Provider = *provider
	if _, ok, err := newCoordinatorClient(cfg); err != nil {
		return err
	} else if ok {
		return exit(2, "machine cleanup is disabled when a coordinator is configured; coordinator TTL alarms own brokered cleanup")
	}
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
			shouldDelete, reason := shouldCleanupServer(s, time.Now().UTC())
			if !shouldDelete {
				fmt.Fprintf(a.Stderr, "skip server id=%s name=%s reason=%s\n", s.DisplayID(), s.Name, reason)
				continue
			}
			fmt.Fprintf(a.Stderr, "delete server id=%s name=%s\n", s.DisplayID(), s.Name)
			if !*dryRun {
				if err := deleteServer(ctx, cfg, s); err != nil {
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
		shouldDelete, reason := shouldCleanupServer(s, time.Now().UTC())
		if !shouldDelete {
			fmt.Fprintf(a.Stderr, "skip server id=%s name=%s reason=%s\n", s.DisplayID(), s.Name, reason)
			continue
		}
		fmt.Fprintf(a.Stderr, "delete server id=%s name=%s\n", s.DisplayID(), s.Name)
		if !*dryRun {
			if err := deleteServer(ctx, cfg, s); err != nil {
				return err
			}
		}
	}
	return nil
}

func shouldCleanupServer(server Server, now time.Time) (bool, string) {
	labels := server.Labels
	if labels == nil {
		return false, "missing labels"
	}
	if strings.EqualFold(labels["keep"], "true") {
		return false, "keep=true"
	}
	state := strings.ToLower(labels["state"])
	switch state {
	case "leased", "ready", "running", "active", "provisioning":
		expiresAt, ok := cleanupExpiry(labels)
		if ok && now.After(expiresAt.Add(12*time.Hour)) {
			return true, "stale state=" + state
		}
		return false, "state=" + state
	}
	if state == "failed" || state == "released" || state == "expired" {
		return true, "state=" + state
	}
	expiresAt, ok := cleanupExpiry(labels)
	if !ok {
		return false, "missing expires_at"
	}
	if now.Before(expiresAt) {
		return false, "not expired"
	}
	return true, "expired"
}

func cleanupExpiry(labels map[string]string) (time.Time, bool) {
	for _, key := range []string{"expires_at", "ttl"} {
		value := strings.TrimSpace(labels[key])
		if value == "" {
			continue
		}
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			return parsed, true
		}
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}
