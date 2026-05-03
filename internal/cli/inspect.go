package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func (a App) inspect(ctx context.Context, args []string) error {
	fs := newFlagSet("inspect", a.Stderr)
	provider := fs.String("provider", defaultConfig().Provider, "provider: hetzner, aws, or ssh")
	id := fs.String("id", "", "lease id or slug")
	jsonOut := fs.Bool("json", false, "print JSON")
	targetFlags := registerTargetFlags(fs, defaultConfig())
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *id == "" && fs.NArg() > 0 {
		*id = fs.Arg(0)
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.Provider = *provider
	if err := applyTargetFlagOverrides(&cfg, fs, targetFlags); err != nil {
		return err
	}
	if *id == "" && !isStaticProvider(cfg.Provider) {
		return exit(2, "usage: crabbox inspect --id <lease-id-or-slug>")
	}
	state, err := a.leaseStatus(ctx, cfg, *id)
	if err != nil {
		return err
	}
	if *jsonOut {
		return json.NewEncoder(a.Stdout).Encode(state)
	}
	fmt.Fprintf(a.Stdout, "id=%s\nslug=%s\nprovider=%s\ntarget=%s\nwindows_mode=%s\nstate=%s\nserver=%s\nhost=%s\nssh=%s -p %s %s@%s\nssh_fallback_ports=%s\nidle_for=%s\nidle_timeout=%s\nlast_touched=%s\nexpires=%s\n", state.ID, blank(state.Slug, "-"), state.Provider, state.TargetOS, blank(state.WindowsMode, "-"), state.State, state.ServerID, state.Host, state.SSHKey, state.SSHPort, state.SSHUser, state.Host, blank(strings.Join(state.SSHFallbackPorts, ","), "-"), blank(state.IdleFor, "-"), blank(state.IdleTimeout, "-"), blank(state.LastTouchedAt, "-"), blank(state.ExpiresAt, "-"))
	for key, value := range state.Labels {
		fmt.Fprintf(a.Stdout, "label.%s=%s\n", key, value)
	}
	return nil
}
