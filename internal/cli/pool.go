package cli

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

func (a App) list(ctx context.Context, args []string) error {
	defaults := defaultConfig()
	fs := newFlagSet("list", a.Stderr)
	provider := fs.String("provider", defaults.Provider, "provider: hetzner, aws, ssh, or blacksmith-testbox")
	jsonOut := fs.Bool("json", false, "print JSON")
	providerFlags := registerProviderFlags(fs, defaults)
	targetFlags := registerTargetFlags(fs, defaults)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.Provider = *provider
	if err := applyProviderFlags(&cfg, fs, providerFlags); err != nil {
		return err
	}
	if err := applyTargetFlagOverrides(&cfg, fs, targetFlags); err != nil {
		return err
	}
	backend, err := loadBackend(cfg, runtimeForApp(a))
	if err != nil {
		return err
	}
	var servers []Server
	switch b := backend.(type) {
	case SSHLeaseBackend:
		servers, err = b.List(ctx, ListRequest{Options: leaseOptionsFromConfig(cfg)})
	case DelegatedRunBackend:
		servers, err = b.List(ctx, ListRequest{Options: leaseOptionsFromConfig(cfg)})
	default:
		return exit(2, "provider=%s does not support list", backend.Spec().Name)
	}
	if err != nil {
		return err
	}
	if *jsonOut {
		if jsonBackend, ok := backend.(JSONListBackend); ok {
			view, err := jsonBackend.ListJSON(ctx, ListRequest{Options: leaseOptionsFromConfig(cfg)})
			if err != nil {
				return err
			}
			return json.NewEncoder(a.Stdout).Encode(view)
		}
		return json.NewEncoder(a.Stdout).Encode(servers)
	}
	renderServerList(a.Stdout, servers)
	return nil
}

func activeCoordinatorLeaseIDs(leases []CoordinatorLease) map[string]struct{} {
	ids := make(map[string]struct{}, len(leases))
	for _, lease := range leases {
		if lease.ID != "" {
			ids[lease.ID] = struct{}{}
		}
	}
	return ids
}

func coordinatorMachineOrphanField(labels map[string]string, activeLeaseIDs map[string]struct{}) string {
	leaseID := labels["lease"]
	if leaseID == "" {
		return " orphan=missing-lease-label"
	}
	if _, ok := activeLeaseIDs[leaseID]; !ok {
		return " orphan=no-active-lease"
	}
	return ""
}

func (a App) cleanup(ctx context.Context, args []string) error {
	defaults := defaultConfig()
	fs := newFlagSet("machine cleanup", a.Stderr)
	provider := fs.String("provider", defaults.Provider, "provider: hetzner or aws")
	dryRun := fs.Bool("dry-run", false, "only print")
	providerFlags := registerProviderFlags(fs, defaults)
	targetFlags := registerTargetFlags(fs, defaults)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.Provider = *provider
	if err := applyProviderFlags(&cfg, fs, providerFlags); err != nil {
		return err
	}
	if err := applyTargetFlagOverrides(&cfg, fs, targetFlags); err != nil {
		return err
	}
	backend, err := loadBackend(cfg, runtimeForApp(a))
	if err != nil {
		return err
	}
	if backendCoordinator(backend) != nil {
		return exit(2, "machine cleanup is disabled when a coordinator is configured; coordinator TTL alarms own brokered cleanup")
	}
	cleaner, ok := backend.(CleanupBackend)
	if !ok {
		return exit(2, "machine cleanup is not supported for provider=%s", cfg.Provider)
	}
	return cleaner.Cleanup(ctx, CleanupRequest{Options: leaseOptionsFromConfig(cfg), DryRun: *dryRun})
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
	case "running", "provisioning":
		expiresAt, ok := cleanupExpiry(labels)
		if ok && now.After(expiresAt.Add(12*time.Hour)) {
			return true, "stale state=" + state
		}
		return false, "state=" + state
	case "leased", "ready", "active":
		expiresAt, ok := cleanupExpiry(labels)
		if ok && now.After(expiresAt) {
			return true, "expired state=" + state
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
		if parsed, ok := parseLeaseLabelTime(value); ok {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func directLeaseExpiresAt(now time.Time, cfg Config) time.Time {
	return directLeaseExpiresAtFrom(now, now, cfg.TTL, cfg.IdleTimeout)
}
