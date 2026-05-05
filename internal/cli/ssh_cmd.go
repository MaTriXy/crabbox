package cli

import (
	"context"
	"fmt"
)

func (a App) ssh(ctx context.Context, args []string) error {
	defaults := defaultConfig()
	fs := newFlagSet("ssh", a.Stderr)
	provider := fs.String("provider", defaults.Provider, "provider: hetzner, aws, or ssh")
	id := fs.String("id", "", "lease id or slug")
	reclaim := fs.Bool("reclaim", false, "claim this lease for the current repo")
	targetFlags := registerTargetFlags(fs, defaults)
	networkFlags := registerNetworkModeFlag(fs, defaults)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	setIDFromFirstArg(fs, id)
	cfg, err := loadLeaseTargetConfig(fs, *provider, targetFlags, networkFlags, leaseTargetConfigOptions{})
	if err != nil {
		return err
	}
	if err := requireLeaseID(*id, "crabbox ssh --id <lease-id-or-slug>", cfg); err != nil {
		return err
	}
	server, target, leaseID, err := a.resolveNetworkLeaseTarget(ctx, cfg, *id, false)
	if err != nil {
		return err
	}
	if err := a.claimAndTouchLeaseTarget(ctx, cfg, server, leaseID, *reclaim); err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "ssh -i %s -p %s %s@%s\n", target.Key, target.Port, target.User, target.Host)
	return nil
}
