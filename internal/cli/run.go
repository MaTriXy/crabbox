package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (a App) warmup(ctx context.Context, args []string) error {
	fs := newFlagSet("warmup", a.Stderr)
	provider := fs.String("provider", defaultConfig().Provider, "provider: hetzner or aws")
	profile := fs.String("profile", defaultConfig().Profile, "profile")
	class := fs.String("class", defaultConfig().Class, "machine class")
	serverType := fs.String("type", getenv("CRABBOX_SERVER_TYPE", ""), "provider server/instance type")
	ttl := fs.Duration("ttl", 90*time.Minute, "lease ttl")
	keep := fs.Bool("keep", true, "keep server after warmup")
	if err := fs.Parse(args); err != nil {
		return exit(2, "%v", err)
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.Provider = *provider
	cfg.Profile = *profile
	cfg.Class = *class
	if flagWasSet(fs, "type") {
		cfg.ServerType = *serverType
	}
	if cfg.ServerType == "" || ((flagWasSet(fs, "provider") || flagWasSet(fs, "class")) && !flagWasSet(fs, "type")) {
		cfg.ServerType = serverTypeForProviderClass(cfg.Provider, *class)
	}
	cfg.TTL = *ttl

	coord, useCoordinator, err := newCoordinatorClient(cfg)
	if err != nil {
		return err
	}
	var server Server
	var target SSHTarget
	var leaseID string
	if useCoordinator {
		server, target, leaseID, err = a.acquireCoordinator(ctx, cfg, coord, *keep)
	} else {
		server, target, leaseID, err = a.acquire(ctx, cfg, *keep)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "leased %s provider=%s server=%s type=%s ip=%s\n", leaseID, cfg.Provider, server.DisplayID(), server.ServerType.Name, target.Host)
	fmt.Fprintf(a.Stdout, "ready ssh=%s@%s:%s workroot=%s\n", target.User, target.Host, target.Port, cfg.WorkRoot)
	return nil
}

func (a App) runCommand(ctx context.Context, args []string) error {
	fs := newFlagSet("run", a.Stderr)
	provider := fs.String("provider", defaultConfig().Provider, "provider: hetzner or aws")
	profile := fs.String("profile", defaultConfig().Profile, "profile")
	class := fs.String("class", defaultConfig().Class, "machine class")
	serverType := fs.String("type", getenv("CRABBOX_SERVER_TYPE", ""), "provider server/instance type")
	ttl := fs.Duration("ttl", 90*time.Minute, "lease ttl")
	leaseIDFlag := fs.String("id", "", "existing lease or server id")
	keep := fs.Bool("keep", false, "keep server after command")
	noSync := fs.Bool("no-sync", false, "skip rsync")
	syncOnly := fs.Bool("sync-only", false, "sync and exit")
	if err := fs.Parse(args); err != nil {
		return exit(2, "%v", err)
	}
	command := fs.Args()
	if len(command) > 0 && command[0] == "--" {
		command = command[1:]
	}
	if len(command) == 0 && !*syncOnly {
		return exit(2, "usage: crabbox run [flags] -- <command...>")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.Provider = *provider
	cfg.Profile = *profile
	cfg.Class = *class
	if flagWasSet(fs, "type") {
		cfg.ServerType = *serverType
	}
	if cfg.ServerType == "" || ((flagWasSet(fs, "provider") || flagWasSet(fs, "class")) && !flagWasSet(fs, "type")) {
		cfg.ServerType = serverTypeForProviderClass(cfg.Provider, *class)
	}
	cfg.TTL = *ttl

	var server Server
	var target SSHTarget
	var leaseID string
	acquired := false
	coord, useCoordinator, err := newCoordinatorClient(cfg)
	if err != nil {
		return err
	}
	if *leaseIDFlag != "" {
		if useCoordinator {
			var lease CoordinatorLease
			lease, err = coord.GetLease(ctx, *leaseIDFlag)
			if err == nil {
				server, target, leaseID = leaseToServerTarget(lease, cfg)
			}
		} else {
			server, target, leaseID, err = a.findLease(ctx, cfg, *leaseIDFlag)
		}
	} else {
		if useCoordinator {
			server, target, leaseID, err = a.acquireCoordinator(ctx, cfg, coord, *keep)
		} else {
			server, target, leaseID, err = a.acquire(ctx, cfg, *keep)
		}
		acquired = true
	}
	if err != nil {
		return err
	}
	if acquired {
		defer func() {
			if !*keep {
				fmt.Fprintf(a.Stderr, "releasing %s server=%s\n", leaseID, server.DisplayID())
				if useCoordinator {
					_, _ = coord.ReleaseLease(context.Background(), leaseID, true)
				} else {
					_ = deleteServer(context.Background(), cfg, server)
				}
			}
		}()
	}

	repo, err := findRepo()
	if err != nil {
		return err
	}
	workdir := filepath.ToSlash(filepath.Join(cfg.WorkRoot, leaseID, repo.Name))
	if !*noSync {
		fmt.Fprintf(a.Stderr, "syncing %s -> %s:%s\n", repo.Root, target.Host, workdir)
		if err := runSSHQuiet(ctx, target, remoteMkdir(workdir)); err != nil {
			return exit(7, "create remote workdir: %v", err)
		}
		if err := rsync(ctx, target, repo.Root, workdir, defaultExcludes(), a.Stdout, a.Stderr); err != nil {
			return exit(6, "rsync failed: %v", err)
		}
		if err := runSSHQuiet(ctx, target, remoteGitHydrate(workdir)); err != nil {
			fmt.Fprintf(a.Stderr, "warning: remote git hydrate failed: %v\n", err)
		}
	}
	if *syncOnly {
		fmt.Fprintf(a.Stdout, "synced %s\n", workdir)
		return nil
	}

	fmt.Fprintf(a.Stderr, "running on %s %s\n", target.Host, strings.Join(command, " "))
	code := runSSHStream(ctx, target, remoteCommand(workdir, allowedEnv(), command), a.Stdout, a.Stderr)
	if code != 0 {
		return ExitError{Code: code, Message: fmt.Sprintf("remote command exited %d", code)}
	}
	return nil
}

func (a App) acquireCoordinator(ctx context.Context, cfg Config, coord *CoordinatorClient, keep bool) (Server, SSHTarget, string, error) {
	publicKey, err := publicKeyFor(cfg.SSHKey)
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	fmt.Fprintf(a.Stderr, "coordinator lease class=%s preferred_type=%s keep=%v\n", cfg.Class, cfg.ServerType, keep)
	lease, err := coord.CreateLease(ctx, cfg, publicKey, keep)
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	server, target, leaseID := leaseToServerTarget(lease, cfg)
	fmt.Fprintf(a.Stderr, "leased %s server=%d type=%s ip=%s via coordinator\n", leaseID, server.ID, server.ServerType.Name, target.Host)
	if err := waitForSSH(ctx, target, a.Stderr); err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	return server, target, leaseID, nil
}

func (a App) acquire(ctx context.Context, cfg Config, keep bool) (Server, SSHTarget, string, error) {
	if cfg.Provider == "aws" {
		return a.acquireAWS(ctx, cfg, keep)
	}
	client, err := newHetznerClient()
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	publicKey, err := publicKeyFor(cfg.SSHKey)
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	if cfg.ProviderKey != "" {
		providerKey, err := client.EnsureSSHKey(ctx, cfg.ProviderKey, publicKey)
		if err != nil {
			return Server{}, SSHTarget{}, "", err
		}
		cfg.ProviderKey = providerKey.Name
	}
	leaseID := newLeaseID()
	fmt.Fprintf(a.Stderr, "provisioning provider=hetzner lease=%s class=%s preferred_type=%s location=%s keep=%v\n", leaseID, cfg.Class, cfg.ServerType, cfg.Location, keep)
	server, cfg, err := client.CreateServerWithFallback(ctx, cfg, publicKey, leaseID, keep, func(format string, args ...any) {
		fmt.Fprintf(a.Stderr, format, args...)
	})
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	fmt.Fprintf(a.Stderr, "provisioned lease=%s server=%d type=%s\n", leaseID, server.ID, cfg.ServerType)
	server, err = waitForServerIP(ctx, client, server.ID)
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	target := SSHTarget{User: cfg.SSHUser, Host: server.PublicNet.IPv4.IP, Key: cfg.SSHKey, Port: cfg.SSHPort}
	if err := waitForSSH(ctx, target, a.Stderr); err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	server.Labels["state"] = "ready"
	if err := client.SetLabels(ctx, server.ID, server.Labels); err != nil {
		fmt.Fprintf(a.Stderr, "warning: set labels: %v\n", err)
	}
	return server, target, leaseID, nil
}

func (a App) acquireAWS(ctx context.Context, cfg Config, keep bool) (Server, SSHTarget, string, error) {
	client, err := newAWSClient(ctx, cfg)
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	publicKey, err := publicKeyFor(cfg.SSHKey)
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	leaseID := newLeaseID()
	fmt.Fprintf(a.Stderr, "provisioning provider=aws lease=%s class=%s preferred_type=%s region=%s keep=%v spot=true\n", leaseID, cfg.Class, cfg.ServerType, cfg.AWSRegion, keep)
	server, cfg, err := client.CreateServerWithFallback(ctx, cfg, publicKey, leaseID, keep, func(format string, args ...any) {
		fmt.Fprintf(a.Stderr, format, args...)
	})
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	fmt.Fprintf(a.Stderr, "provisioned lease=%s server=%s type=%s\n", leaseID, server.DisplayID(), cfg.ServerType)
	server, err = client.waitForServerIP(ctx, server.CloudID)
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	target := SSHTarget{User: cfg.SSHUser, Host: server.PublicNet.IPv4.IP, Key: cfg.SSHKey, Port: cfg.SSHPort}
	if err := waitForSSH(ctx, target, a.Stderr); err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	server.Labels["state"] = "ready"
	if err := client.SetTags(ctx, server.CloudID, server.Labels); err != nil {
		fmt.Fprintf(a.Stderr, "warning: set tags: %v\n", err)
	}
	return server, target, leaseID, nil
}

func waitForServerIP(ctx context.Context, client *HetznerClient, id int64) (Server, error) {
	deadline := time.Now().Add(5 * time.Minute)
	for {
		server, err := client.GetServer(ctx, id)
		if err != nil {
			return Server{}, err
		}
		if server.PublicNet.IPv4.IP != "" {
			return server, nil
		}
		if time.Now().After(deadline) {
			return Server{}, exit(5, "timed out waiting for server IP")
		}
		time.Sleep(3 * time.Second)
	}
}

func (a App) findLease(ctx context.Context, cfg Config, id string) (Server, SSHTarget, string, error) {
	if cfg.Provider == "aws" {
		return a.findAWSLease(ctx, cfg, id)
	}
	client, err := newHetznerClient()
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	if serverID, ok := parseServerID(id); ok {
		server, err := client.GetServer(ctx, serverID)
		if err != nil {
			return Server{}, SSHTarget{}, "", err
		}
		leaseID := server.Labels["lease"]
		if leaseID == "" {
			leaseID = id
		}
		return server, SSHTarget{User: cfg.SSHUser, Host: server.PublicNet.IPv4.IP, Key: cfg.SSHKey, Port: cfg.SSHPort}, leaseID, nil
	}
	servers, err := client.ListCrabboxServers(ctx)
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	for _, server := range servers {
		if server.Labels["lease"] == id || server.Name == id {
			return server, SSHTarget{User: cfg.SSHUser, Host: server.PublicNet.IPv4.IP, Key: cfg.SSHKey, Port: cfg.SSHPort}, server.Labels["lease"], nil
		}
	}
	return Server{}, SSHTarget{}, "", exit(4, "lease/server not found: %s", id)
}

func (a App) findAWSLease(ctx context.Context, cfg Config, id string) (Server, SSHTarget, string, error) {
	client, err := newAWSClient(ctx, cfg)
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	if strings.HasPrefix(id, "i-") {
		server, err := client.GetServer(ctx, id)
		if err != nil {
			return Server{}, SSHTarget{}, "", err
		}
		leaseID := server.Labels["lease"]
		if leaseID == "" {
			leaseID = id
		}
		return server, SSHTarget{User: cfg.SSHUser, Host: server.PublicNet.IPv4.IP, Key: cfg.SSHKey, Port: cfg.SSHPort}, leaseID, nil
	}
	servers, err := client.ListCrabboxServers(ctx)
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	for _, server := range servers {
		if server.Labels["lease"] == id || server.Name == id {
			return server, SSHTarget{User: cfg.SSHUser, Host: server.PublicNet.IPv4.IP, Key: cfg.SSHKey, Port: cfg.SSHPort}, server.Labels["lease"], nil
		}
	}
	return Server{}, SSHTarget{}, "", exit(4, "lease/server not found: %s", id)
}

func (a App) stop(ctx context.Context, args []string) error {
	fs := newFlagSet("stop", a.Stderr)
	provider := fs.String("provider", defaultConfig().Provider, "provider: hetzner or aws")
	if err := fs.Parse(args); err != nil {
		return exit(2, "%v", err)
	}
	if fs.NArg() != 1 {
		return exit(2, "usage: crabbox stop <lease-or-server-id>")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.Provider = *provider
	if coord, ok, err := newCoordinatorClient(cfg); err != nil {
		return err
	} else if ok {
		lease, err := coord.ReleaseLease(ctx, fs.Arg(0), true)
		if err != nil {
			return err
		}
		fmt.Fprintf(a.Stderr, "released lease=%s server=%s\n", lease.ID, leaseDisplayID(lease))
		return nil
	}
	server, _, leaseID, err := a.findLease(ctx, cfg, fs.Arg(0))
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Stderr, "deleting lease=%s server=%s name=%s\n", leaseID, server.DisplayID(), server.Name)
	return deleteServer(ctx, cfg, server)
}

func leaseDisplayID(lease CoordinatorLease) string {
	if lease.CloudID != "" {
		return lease.CloudID
	}
	return fmt.Sprint(lease.ServerID)
}

func deleteServer(ctx context.Context, cfg Config, server Server) error {
	if cfg.Provider == "aws" || server.Provider == "aws" || strings.HasPrefix(server.CloudID, "i-") {
		client, err := newAWSClient(ctx, cfg)
		if err != nil {
			return err
		}
		return client.DeleteServer(ctx, server.CloudID)
	}
	client, err := newHetznerClient()
	if err != nil {
		return err
	}
	return client.DeleteServer(ctx, server.ID)
}

func repoExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
