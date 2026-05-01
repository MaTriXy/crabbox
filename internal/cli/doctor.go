package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

func (a App) doctor(ctx context.Context, args []string) error {
	fs := newFlagSet("doctor", a.Stderr)
	provider := fs.String("provider", defaultConfig().Provider, "provider: hetzner or aws")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	ok := true
	for _, tool := range []string{"git", "ssh", "ssh-keygen", "rsync", "curl"} {
		path, err := exec.LookPath(tool)
		if err != nil {
			fmt.Fprintf(a.Stdout, "missing %-8s\n", tool)
			ok = false
			continue
		}
		fmt.Fprintf(a.Stdout, "ok      %-8s %s\n", tool, path)
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.Provider = *provider
	if os.Getenv("CRABBOX_SERVER_TYPE") == "" {
		cfg.ServerType = serverTypeForProviderClass(cfg.Provider, cfg.Class)
	}
	useCoordinator := false
	if coord, coordinatorConfigured, err := newCoordinatorClient(cfg); err != nil {
		fmt.Fprintf(a.Stdout, "failed  coord    %v\n", err)
		ok = false
	} else if coordinatorConfigured {
		if err := coord.Health(ctx); err != nil {
			fmt.Fprintf(a.Stdout, "failed  coord    %v\n", err)
			ok = false
		} else {
			fmt.Fprintf(a.Stdout, "ok      coord    %s\n", cfg.Coordinator)
			useCoordinator = true
			if machines, err := coord.Pool(ctx, cfg); err != nil {
				fmt.Fprintf(a.Stdout, "failed  broker   %v\n", err)
				ok = false
			} else {
				fmt.Fprintf(a.Stdout, "ok      broker   provider=%s machines=%d default_type=%s\n", cfg.Provider, len(machines), cfg.ServerType)
			}
		}
	}

	if os.Getenv("CRABBOX_SSH_KEY") != "" {
		if _, err := os.Stat(cfg.SSHKey); err != nil {
			fmt.Fprintf(a.Stdout, "missing ssh key %s\n", cfg.SSHKey)
			ok = false
		} else if _, err := publicKeyFor(cfg.SSHKey); err != nil {
			fmt.Fprintf(a.Stdout, "missing ssh public key %s.pub\n", cfg.SSHKey)
			ok = false
		} else {
			fmt.Fprintf(a.Stdout, "ok      ssh-key  %s\n", cfg.SSHKey)
		}
	} else {
		fmt.Fprintf(a.Stdout, "ok      ssh-key  per-lease\n")
	}

	if useCoordinator {
		if !ok {
			return exit(1, "doctor found problems")
		}
		return nil
	}

	switch cfg.Provider {
	case "aws":
		client, err := newAWSClient(ctx, cfg)
		if err != nil {
			fmt.Fprintf(a.Stdout, "failed  aws      %v\n", err)
			ok = false
			break
		}
		servers, err := client.ListCrabboxServers(ctx)
		if err != nil {
			fmt.Fprintf(a.Stdout, "failed  aws      %v\n", err)
			ok = false
		} else {
			fmt.Fprintf(a.Stdout, "ok      aws      crabbox_servers=%d region=%s default_type=%s\n", len(servers), cfg.AWSRegion, cfg.ServerType)
		}
	default:
		client, err := newHetznerClient()
		if err != nil {
			fmt.Fprintf(a.Stdout, "missing hcloud token\n")
			ok = false
		} else {
			servers, err := client.ListCrabboxServers(ctx)
			if err != nil {
				fmt.Fprintf(a.Stdout, "failed  hcloud   %v\n", err)
				ok = false
			} else {
				fmt.Fprintf(a.Stdout, "ok      hcloud   crabbox_servers=%d default_type=%s\n", len(servers), cfg.ServerType)
			}
		}
	}

	if !ok {
		return exit(1, "doctor found problems")
	}
	return nil
}
