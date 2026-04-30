package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

func (a App) doctor(ctx context.Context, args []string) error {
	fs := newFlagSet("doctor", a.Stderr)
	if err := fs.Parse(args); err != nil {
		return exit(2, "%v", err)
	}

	ok := true
	for _, tool := range []string{"git", "ssh", "rsync", "curl"} {
		path, err := exec.LookPath(tool)
		if err != nil {
			fmt.Fprintf(a.Stdout, "missing %-8s\n", tool)
			ok = false
			continue
		}
		fmt.Fprintf(a.Stdout, "ok      %-8s %s\n", tool, path)
	}

	cfg := defaultConfig()
	if _, err := os.Stat(cfg.SSHKey); err != nil {
		fmt.Fprintf(a.Stdout, "missing ssh key %s\n", cfg.SSHKey)
		ok = false
	} else if _, err := publicKeyFor(cfg.SSHKey); err != nil {
		fmt.Fprintf(a.Stdout, "missing ssh public key %s.pub\n", cfg.SSHKey)
		ok = false
	} else {
		fmt.Fprintf(a.Stdout, "ok      ssh-key  %s\n", cfg.SSHKey)
	}

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

	if !ok {
		return exit(1, "doctor found problems")
	}
	return nil
}
