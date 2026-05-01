package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func (a App) status(ctx context.Context, args []string) error {
	fs := newFlagSet("status", a.Stderr)
	provider := fs.String("provider", defaultConfig().Provider, "provider: hetzner or aws")
	id := fs.String("id", "", "lease id")
	wait := fs.Bool("wait", false, "wait until ready")
	waitTimeout := fs.Duration("wait-timeout", 5*time.Minute, "maximum wait duration")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *id == "" && fs.NArg() > 0 {
		*id = fs.Arg(0)
	}
	if *id == "" {
		return exit(2, "usage: crabbox status --id <lease-id>")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.Provider = *provider
	deadline := time.Now().Add(*waitTimeout)
	for {
		state, err := a.leaseStatus(ctx, cfg, *id)
		if err != nil {
			return err
		}
		if *jsonOut {
			if !*wait || state.Ready {
				return json.NewEncoder(a.Stdout).Encode(state)
			}
		} else {
			fmt.Fprintf(a.Stdout, "%s provider=%s state=%s type=%s host=%s expires=%s\n", state.ID, state.Provider, state.State, state.ServerType, state.Host, blank(state.ExpiresAt, "-"))
		}
		if !*wait || state.Ready {
			return nil
		}
		if time.Now().After(deadline) {
			return exit(5, "timed out waiting for %s to become ready", *id)
		}
		time.Sleep(5 * time.Second)
	}
}

func (a App) ssh(ctx context.Context, args []string) error {
	fs := newFlagSet("ssh", a.Stderr)
	provider := fs.String("provider", defaultConfig().Provider, "provider: hetzner or aws")
	id := fs.String("id", "", "lease id")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *id == "" && fs.NArg() > 0 {
		*id = fs.Arg(0)
	}
	if *id == "" {
		return exit(2, "usage: crabbox ssh --id <lease-id>")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.Provider = *provider
	_, target, _, err := a.resolveLeaseTarget(ctx, cfg, *id)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "ssh -i %s -p %s %s@%s\n", target.Key, target.Port, target.User, target.Host)
	return nil
}

func (a App) inspect(ctx context.Context, args []string) error {
	fs := newFlagSet("inspect", a.Stderr)
	provider := fs.String("provider", defaultConfig().Provider, "provider: hetzner or aws")
	id := fs.String("id", "", "lease id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *id == "" && fs.NArg() > 0 {
		*id = fs.Arg(0)
	}
	if *id == "" {
		return exit(2, "usage: crabbox inspect --id <lease-id>")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.Provider = *provider
	state, err := a.leaseStatus(ctx, cfg, *id)
	if err != nil {
		return err
	}
	if *jsonOut {
		return json.NewEncoder(a.Stdout).Encode(state)
	}
	fmt.Fprintf(a.Stdout, "id=%s\nprovider=%s\nstate=%s\nserver=%s\nhost=%s\nssh=%s -p %s %s@%s\nexpires=%s\n", state.ID, state.Provider, state.State, state.ServerID, state.Host, state.SSHKey, state.SSHPort, state.SSHUser, state.Host, blank(state.ExpiresAt, "-"))
	for key, value := range state.Labels {
		fmt.Fprintf(a.Stdout, "label.%s=%s\n", key, value)
	}
	return nil
}

type statusView struct {
	ID         string            `json:"id"`
	Provider   string            `json:"provider"`
	State      string            `json:"state"`
	ServerID   string            `json:"serverId"`
	ServerType string            `json:"serverType"`
	Host       string            `json:"host"`
	SSHUser    string            `json:"sshUser"`
	SSHPort    string            `json:"sshPort"`
	SSHKey     string            `json:"sshKey"`
	ExpiresAt  string            `json:"expiresAt,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Ready      bool              `json:"ready"`
}

func (a App) leaseStatus(ctx context.Context, cfg Config, id string) (statusView, error) {
	if coord, ok, err := newCoordinatorClient(cfg); err != nil {
		return statusView{}, err
	} else if ok {
		lease, err := coord.GetLease(ctx, id)
		if err != nil {
			return statusView{}, err
		}
		_, target, _ := leaseToServerTarget(lease, cfg)
		return statusView{
			ID:         lease.ID,
			Provider:   blank(lease.Provider, cfg.Provider),
			State:      lease.State,
			ServerID:   leaseDisplayID(lease),
			ServerType: lease.ServerType,
			Host:       lease.Host,
			SSHUser:    target.User,
			SSHPort:    target.Port,
			SSHKey:     target.Key,
			ExpiresAt:  lease.ExpiresAt,
			Labels:     map[string]string{"keep": fmt.Sprint(lease.Keep)},
			Ready:      lease.State == "active" && lease.Host != "",
		}, nil
	}
	server, target, leaseID, err := a.findLease(ctx, cfg, id)
	if err != nil {
		return statusView{}, err
	}
	return statusView{
		ID:         leaseID,
		Provider:   blank(server.Provider, cfg.Provider),
		State:      blank(server.Labels["state"], server.Status),
		ServerID:   server.DisplayID(),
		ServerType: server.ServerType.Name,
		Host:       server.PublicNet.IPv4.IP,
		SSHUser:    target.User,
		SSHPort:    target.Port,
		SSHKey:     target.Key,
		ExpiresAt:  server.Labels["expires_at"],
		Labels:     server.Labels,
		Ready:      server.PublicNet.IPv4.IP != "" && server.Labels["state"] != "provisioning",
	}, nil
}

func (a App) resolveLeaseTarget(ctx context.Context, cfg Config, id string) (Server, SSHTarget, string, error) {
	if coord, ok, err := newCoordinatorClient(cfg); err != nil {
		return Server{}, SSHTarget{}, "", err
	} else if ok {
		lease, err := coord.GetLease(ctx, id)
		if err != nil {
			return Server{}, SSHTarget{}, "", err
		}
		server, target, leaseID := leaseToServerTarget(lease, cfg)
		return server, target, leaseID, nil
	}
	return a.findLease(ctx, cfg, id)
}
