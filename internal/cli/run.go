package cli

import (
	"context"
	"fmt"
	"io"
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
	idleTimeout := fs.Duration("idle-timeout", 0, "idle timeout")
	keep := fs.Bool("keep", true, "keep server after warmup")
	actionsRunner := fs.Bool("actions-runner", false, "register this box as an ephemeral GitHub Actions runner")
	if err := parseFlags(fs, args); err != nil {
		return err
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
	if flagWasSet(fs, "idle-timeout") {
		cfg.TTL = *idleTimeout
	}
	if cfg.TTL <= 0 {
		return exit(2, "idle timeout must be positive")
	}

	coord, useCoordinator, err := newCoordinatorClient(cfg)
	if err != nil {
		return err
	}
	var server Server
	var target SSHTarget
	var leaseID string
	if useCoordinator {
		server, target, leaseID, err = a.acquireCoordinatorWithRetry(ctx, cfg, coord, *keep)
	} else {
		server, target, leaseID, err = a.acquireWithRetry(ctx, cfg, *keep)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "leased %s provider=%s server=%s type=%s ip=%s\n", leaseID, cfg.Provider, server.DisplayID(), server.ServerType.Name, target.Host)
	fmt.Fprintf(a.Stdout, "ready ssh=%s@%s:%s workroot=%s\n", target.User, target.Host, target.Port, cfg.WorkRoot)
	if *actionsRunner {
		repo, err := findRepo()
		if err != nil {
			return err
		}
		ghRepo, err := resolveGitHubRepo(repo, cfg.Actions.Repo)
		if err != nil {
			return err
		}
		if err := a.registerGitHubActionsRunner(ctx, cfg, target, leaseID, ghRepo, "", nil); err != nil {
			return err
		}
	}
	return nil
}

func (a App) runCommand(ctx context.Context, args []string) error {
	fs := newFlagSet("run", a.Stderr)
	provider := fs.String("provider", defaultConfig().Provider, "provider: hetzner or aws")
	profile := fs.String("profile", defaultConfig().Profile, "profile")
	class := fs.String("class", defaultConfig().Class, "machine class")
	serverType := fs.String("type", getenv("CRABBOX_SERVER_TYPE", ""), "provider server/instance type")
	ttl := fs.Duration("ttl", 90*time.Minute, "lease ttl")
	idleTimeout := fs.Duration("idle-timeout", 0, "idle timeout")
	leaseIDFlag := fs.String("id", "", "existing lease or server id")
	keep := fs.Bool("keep", false, "keep server after command")
	noSync := fs.Bool("no-sync", false, "skip rsync")
	syncOnly := fs.Bool("sync-only", false, "sync and exit")
	debugSync := fs.Bool("debug", false, "print detailed sync timing")
	shellMode := fs.Bool("shell", false, "run command through the remote shell")
	checksumSync := fs.Bool("checksum", false, "use checksum rsync instead of size/time")
	junitResults := fs.String("junit", "", "comma-separated remote JUnit XML paths to record")
	if err := parseFlags(fs, args); err != nil {
		return err
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
	if flagWasSet(fs, "idle-timeout") {
		cfg.TTL = *idleTimeout
	}
	if flagWasSet(fs, "checksum") {
		cfg.Sync.Checksum = *checksumSync
	}
	if *junitResults != "" {
		cfg.Results.JUnit = splitCommaList(*junitResults)
	}
	if cfg.TTL <= 0 {
		return exit(2, "idle timeout must be positive")
	}

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
			server, target, leaseID, err = a.acquireCoordinatorWithRetry(ctx, cfg, coord, *keep)
		} else {
			server, target, leaseID, err = a.acquireWithRetry(ctx, cfg, *keep)
		}
		acquired = true
	}
	if err != nil {
		return err
	}
	if acquired {
		defer func() {
			if !*keep {
				a.writeActionsHydrationStopBestEffort(context.Background(), target, leaseID)
				fmt.Fprintf(a.Stderr, "releasing %s server=%s\n", leaseID, server.DisplayID())
				if useCoordinator {
					if err := releaseCoordinatorLease(context.Background(), coord, leaseID); err != nil {
						fmt.Fprintf(a.Stderr, "warning: release failed for %s: %v\n", leaseID, err)
					}
				} else {
					_ = deleteServer(context.Background(), cfg, server)
				}
			}
		}()
	}
	if useCoordinator && leaseID != "" {
		stopHeartbeat := startCoordinatorHeartbeat(ctx, coord, leaseID, cfg.TTL, a.Stderr)
		defer stopHeartbeat()
	}

	repo, err := findRepo()
	if err != nil {
		return err
	}
	if cfg.Sync.BaseRef == "" {
		cfg.Sync.BaseRef = repo.BaseRef
	}
	timings := runTimings{started: time.Now()}
	workdir := filepath.ToSlash(filepath.Join(cfg.WorkRoot, leaseID, repo.Name))
	actionsEnvFile := ""
	if state, err := readActionsHydrationState(ctx, target, leaseID); err == nil && state.Workspace != "" {
		workdir = state.Workspace
		actionsEnvFile = state.EnvFile
		fmt.Fprintf(a.Stderr, "using GitHub Actions workspace %s\n", workdir)
	}
	if !*noSync {
		syncStart := time.Now()
		fmt.Fprintf(a.Stderr, "syncing %s -> %s:%s\n", repo.Root, target.Host, workdir)
		if err := waitForSSHReady(ctx, &target, a.Stderr, "before sync", 2*time.Minute); err != nil {
			return err
		}
		if err := runSSHQuiet(ctx, target, remoteMkdir(workdir)); err != nil {
			return exit(7, "create remote workdir: %v", err)
		}
		fingerprint := ""
		if cfg.Sync.Fingerprint {
			fingerprint, err = syncFingerprint(repo, cfg)
			if err != nil {
				fmt.Fprintf(a.Stderr, "warning: sync fingerprint failed: %v\n", err)
			} else if fingerprint != "" {
				remoteFingerprint, err := runSSHOutput(ctx, target, remoteReadSyncFingerprint(workdir))
				if err == nil && remoteFingerprint == fingerprint {
					timings.sync = time.Since(syncStart)
					fmt.Fprintf(a.Stderr, "No changes detected, skipping sync (%s)\n", timings.sync.Round(time.Millisecond))
					goto afterSync
				}
			}
		}
		if cfg.Sync.GitSeed {
			if err := runSSHQuiet(ctx, target, remoteGitSeed(workdir, repo.RemoteURL, repo.Head)); err != nil {
				fmt.Fprintf(a.Stderr, "warning: remote git seed failed: %v\n", err)
			}
		}
		if err := rsync(ctx, target, repo.Root, workdir, configuredExcludes(cfg), a.Stdout, a.Stderr, rsyncOptions{Debug: *debugSync, Delete: cfg.Sync.Delete, Checksum: cfg.Sync.Checksum}); err != nil {
			return exit(6, "rsync failed: %v", err)
		}
		if err := runSSHQuiet(ctx, target, remoteSyncSanity(workdir, os.Getenv("CRABBOX_ALLOW_MASS_DELETIONS") == "1")); err != nil {
			return exit(6, "remote sync sanity failed: %v", err)
		}
		if err := runSSHQuiet(ctx, target, remoteGitHydrate(workdir, cfg.Sync.BaseRef)); err != nil {
			fmt.Fprintf(a.Stderr, "warning: remote git hydrate failed: %v\n", err)
		}
		if fingerprint != "" {
			if err := runSSHQuiet(ctx, target, remoteWriteSyncFingerprint(workdir, fingerprint)); err != nil {
				fmt.Fprintf(a.Stderr, "warning: write sync fingerprint failed: %v\n", err)
			}
		}
		timings.sync = time.Since(syncStart)
		fmt.Fprintf(a.Stderr, "sync complete in %s\n", timings.sync.Round(time.Millisecond))
	}
afterSync:
	if *syncOnly {
		fmt.Fprintf(a.Stdout, "synced %s\n", workdir)
		return nil
	}

	commandStart := time.Now()
	if err := waitForSSHReady(ctx, &target, a.Stderr, "before command", 2*time.Minute); err != nil {
		return err
	}
	if !useCoordinator {
		setServerState(context.Background(), cfg, server, "running", a.Stderr)
		defer setServerState(context.Background(), cfg, server, "ready", a.Stderr)
	}
	fmt.Fprintf(a.Stderr, "running on %s %s\n", target.Host, strings.Join(command, " "))
	var runID string
	if useCoordinator && leaseID != "" && coord != nil {
		run, err := coord.CreateRun(ctx, leaseID, cfg, command)
		if err != nil {
			fmt.Fprintf(a.Stderr, "warning: run history create failed: %v\n", err)
		} else {
			runID = run.ID
			fmt.Fprintf(a.Stderr, "recording run %s\n", runID)
		}
	}
	remote := remoteCommandWithEnvFile(workdir, allowedEnv(cfg.EnvAllow), actionsEnvFile, command)
	if *shellMode || shouldUseShell(command) {
		remote = remoteShellCommandWithEnvFile(workdir, allowedEnv(cfg.EnvAllow), actionsEnvFile, strings.Join(command, " "))
	}
	var logBuffer runLogBuffer
	stdout := io.MultiWriter(a.Stdout, &logBuffer)
	stderr := io.MultiWriter(a.Stderr, &logBuffer)
	code := runSSHStream(ctx, target, remote, stdout, stderr)
	timings.command = time.Since(commandStart)
	var results *TestResultSummary
	if len(cfg.Results.JUnit) > 0 {
		results, err = collectRemoteJUnitResults(ctx, target, workdir, cfg.Results.JUnit)
		if err != nil {
			fmt.Fprintf(a.Stderr, "warning: collect test results failed: %v\n", err)
		} else if line := resultSummaryLine(results); line != "" {
			fmt.Fprintln(a.Stderr, line)
		}
	}
	if runID != "" {
		if _, err := coord.FinishRun(context.Background(), runID, code, timings.sync, timings.command, logBuffer.String(), logBuffer.Truncated(), results); err != nil {
			fmt.Fprintf(a.Stderr, "warning: run history finish failed for %s: %v\n", runID, err)
		}
	}
	fmt.Fprintf(a.Stderr, "command complete in %s total=%s\n", timings.command.Round(time.Millisecond), time.Since(timings.started).Round(time.Millisecond))
	if code != 0 {
		return ExitError{Code: code, Message: fmt.Sprintf("remote command exited %d", code)}
	}
	return nil
}

type runTimings struct {
	started time.Time
	sync    time.Duration
	command time.Duration
}

func shouldUseShell(command []string) bool {
	if len(command) == 1 {
		return strings.ContainsAny(command[0], "&|;<>*$`")
	}
	for _, word := range command {
		switch word {
		case "&&", "||", ";", "|", ">", ">>", "<", "2>", "2>>":
			return true
		}
	}
	return false
}

func (a App) acquireCoordinator(ctx context.Context, cfg Config, coord *CoordinatorClient, keep bool) (Server, SSHTarget, string, error) {
	leaseID := newLeaseID()
	keyPath, publicKey, err := ensureTestboxKey(leaseID)
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	cfg.SSHKey = keyPath
	cfg.ProviderKey = providerKeyForLease(leaseID)
	fmt.Fprintf(a.Stderr, "coordinator lease class=%s preferred_type=%s keep=%v\n", cfg.Class, cfg.ServerType, keep)
	lease, err := coord.CreateLease(ctx, cfg, publicKey, keep, leaseID)
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	if lease.ID != "" && lease.ID != leaseID {
		if err := moveStoredTestboxKey(leaseID, lease.ID); err != nil {
			fmt.Fprintf(a.Stderr, "warning: could not move local key from %s to %s: %v\n", leaseID, lease.ID, err)
		}
	}
	server, target, leaseID := leaseToServerTarget(lease, cfg)
	fmt.Fprintf(a.Stderr, "leased %s server=%d type=%s ip=%s via coordinator\n", leaseID, server.ID, server.ServerType.Name, target.Host)
	if err := waitForSSH(ctx, &target, a.Stderr); err != nil {
		if !keep {
			if releaseErr := releaseCoordinatorLease(context.Background(), coord, leaseID); releaseErr != nil {
				fmt.Fprintf(a.Stderr, "warning: release failed after bootstrap error for %s: %v\n", leaseID, releaseErr)
			}
		}
		return Server{}, SSHTarget{}, "", err
	}
	return server, target, leaseID, nil
}

func (a App) acquireCoordinatorWithRetry(ctx context.Context, cfg Config, coord *CoordinatorClient, keep bool) (Server, SSHTarget, string, error) {
	var lastErr error
	attempts := acquireAttempts(keep)
	for attempt := 1; attempt <= attempts; attempt++ {
		server, target, leaseID, err := a.acquireCoordinator(ctx, cfg, coord, keep)
		if err == nil {
			return server, target, leaseID, nil
		}
		lastErr = err
		if attempt == attempts || !isBootstrapWaitError(err) {
			return Server{}, SSHTarget{}, "", err
		}
		fmt.Fprintf(a.Stderr, "warning: bootstrap failed; retrying with fresh lease: %v\n", err)
	}
	return Server{}, SSHTarget{}, "", lastErr
}

func (a App) acquireWithRetry(ctx context.Context, cfg Config, keep bool) (Server, SSHTarget, string, error) {
	var lastErr error
	attempts := acquireAttempts(keep)
	for attempt := 1; attempt <= attempts; attempt++ {
		server, target, leaseID, err := a.acquire(ctx, cfg, keep)
		if err == nil {
			return server, target, leaseID, nil
		}
		lastErr = err
		if attempt == attempts || !isBootstrapWaitError(err) {
			return Server{}, SSHTarget{}, "", err
		}
		fmt.Fprintf(a.Stderr, "warning: bootstrap failed; retrying with fresh lease: %v\n", err)
	}
	return Server{}, SSHTarget{}, "", lastErr
}

func acquireAttempts(keep bool) int {
	if keep {
		return 1
	}
	return 2
}

func isBootstrapWaitError(err error) bool {
	var exitErr ExitError
	return AsExitError(err, &exitErr) &&
		exitErr.Code == 5 &&
		strings.Contains(exitErr.Message, "timed out waiting for SSH")
}

func releaseCoordinatorLease(ctx context.Context, coord *CoordinatorClient, leaseID string) error {
	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		releaseCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		_, err := coord.ReleaseLease(releaseCtx, leaseID, true)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt == 5 {
			break
		}
		time.Sleep(time.Duration(attempt*2) * time.Second)
	}
	return lastErr
}

func startCoordinatorHeartbeat(ctx context.Context, coord *CoordinatorClient, leaseID string, ttl time.Duration, stderr io.Writer) func() {
	rootCtx, cancel := context.WithCancel(ctx)
	interval := heartbeatInterval(ttl)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			callCtx, heartbeatCancel := context.WithTimeout(rootCtx, 20*time.Second)
			_, err := coord.HeartbeatLease(callCtx, leaseID)
			heartbeatCancel()
			if err != nil && rootCtx.Err() == nil {
				fmt.Fprintf(stderr, "warning: heartbeat failed for %s: %v\n", leaseID, err)
			}
			select {
			case <-ticker.C:
			case <-rootCtx.Done():
				return
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func heartbeatInterval(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return time.Minute
	}
	interval := ttl / 3
	if interval < 5*time.Second {
		return 5 * time.Second
	}
	if interval > time.Minute {
		return time.Minute
	}
	return interval
}

func setServerState(ctx context.Context, cfg Config, server Server, state string, stderr io.Writer) {
	if server.Labels == nil {
		server.Labels = map[string]string{}
	}
	server.Labels["state"] = state
	if cfg.Provider == "aws" || server.Provider == "aws" || strings.HasPrefix(server.CloudID, "i-") {
		client, err := newAWSClient(ctx, cfg)
		if err != nil {
			fmt.Fprintf(stderr, "warning: set server state=%s: %v\n", state, err)
			return
		}
		if err := client.SetTags(ctx, server.CloudID, server.Labels); err != nil {
			fmt.Fprintf(stderr, "warning: set server state=%s: %v\n", state, err)
		}
		return
	}
	client, err := newHetznerClient()
	if err != nil {
		fmt.Fprintf(stderr, "warning: set server state=%s: %v\n", state, err)
		return
	}
	if err := client.SetLabels(ctx, server.ID, server.Labels); err != nil {
		fmt.Fprintf(stderr, "warning: set server state=%s: %v\n", state, err)
	}
}

func (a App) acquire(ctx context.Context, cfg Config, keep bool) (Server, SSHTarget, string, error) {
	if cfg.Provider == "aws" {
		return a.acquireAWS(ctx, cfg, keep)
	}
	client, err := newHetznerClient()
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	leaseID := newLeaseID()
	keyPath, publicKey, err := ensureTestboxKey(leaseID)
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	cfg.SSHKey = keyPath
	cfg.ProviderKey = providerKeyForLease(leaseID)
	if cfg.ProviderKey != "" {
		providerKey, err := client.EnsureSSHKey(ctx, cfg.ProviderKey, publicKey)
		if err != nil {
			return Server{}, SSHTarget{}, "", err
		}
		cfg.ProviderKey = providerKey.Name
	}
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
	if err := waitForSSH(ctx, &target, a.Stderr); err != nil {
		if !keep {
			_ = deleteServer(context.Background(), cfg, server)
		}
		return Server{}, SSHTarget{}, "", err
	}
	server.Labels["state"] = "ready"
	if err := client.SetLabels(ctx, server.ID, server.Labels); err != nil {
		fmt.Fprintf(a.Stderr, "warning: set labels: %v\n", err)
	}
	return server, target, leaseID, nil
}

func (a App) acquireAWS(ctx context.Context, cfg Config, keep bool) (Server, SSHTarget, string, error) {
	cfg = a.chooseAWSRegion(ctx, cfg)
	client, err := newAWSClient(ctx, cfg)
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	leaseID := newLeaseID()
	keyPath, publicKey, err := ensureTestboxKey(leaseID)
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	cfg.SSHKey = keyPath
	cfg.ProviderKey = providerKeyForLease(leaseID)
	fmt.Fprintf(a.Stderr, "provisioning provider=aws lease=%s class=%s preferred_type=%s region=%s keep=%v market=%s strategy=%s\n", leaseID, cfg.Class, cfg.ServerType, cfg.AWSRegion, keep, cfg.Capacity.Market, cfg.Capacity.Strategy)
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
	if err := waitForSSH(ctx, &target, a.Stderr); err != nil {
		if !keep {
			_ = client.DeleteServer(context.Background(), server.CloudID)
		}
		return Server{}, SSHTarget{}, "", err
	}
	server.Labels["state"] = "ready"
	if err := client.SetTags(ctx, server.CloudID, server.Labels); err != nil {
		fmt.Fprintf(a.Stderr, "warning: set tags: %v\n", err)
	}
	return server, target, leaseID, nil
}

func (a App) chooseAWSRegion(ctx context.Context, cfg Config) Config {
	if cfg.Provider != "aws" || cfg.Capacity.Market != "spot" || len(cfg.Capacity.Regions) < 2 {
		return cfg
	}
	client, err := newAWSClient(ctx, cfg)
	if err != nil {
		fmt.Fprintf(a.Stderr, "warning: spot placement score unavailable: %v\n", err)
		return cfg
	}
	scores, err := client.SpotPlacementScores(ctx, cfg)
	if err != nil {
		fmt.Fprintf(a.Stderr, "warning: spot placement score unavailable: %v\n", err)
		return cfg
	}
	if len(scores) == 0 {
		return cfg
	}
	best := awsString(scores[0].Region)
	score := int32(0)
	if scores[0].Score != nil {
		score = *scores[0].Score
	}
	if best != "" && best != cfg.AWSRegion {
		fmt.Fprintf(a.Stderr, "selected aws region=%s spot_score=%d previous=%s\n", best, score, cfg.AWSRegion)
		cfg.AWSRegion = best
	}
	return cfg
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
		target := SSHTarget{User: cfg.SSHUser, Host: server.PublicNet.IPv4.IP, Key: cfg.SSHKey, Port: cfg.SSHPort}
		useStoredTestboxKey(&target, leaseID)
		return server, target, leaseID, nil
	}
	servers, err := client.ListCrabboxServers(ctx)
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	for _, server := range servers {
		if server.Labels["lease"] == id || server.Name == id {
			leaseID := server.Labels["lease"]
			target := SSHTarget{User: cfg.SSHUser, Host: server.PublicNet.IPv4.IP, Key: cfg.SSHKey, Port: cfg.SSHPort}
			useStoredTestboxKey(&target, leaseID)
			return server, target, leaseID, nil
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
		target := SSHTarget{User: cfg.SSHUser, Host: server.PublicNet.IPv4.IP, Key: cfg.SSHKey, Port: cfg.SSHPort}
		useStoredTestboxKey(&target, leaseID)
		return server, target, leaseID, nil
	}
	servers, err := client.ListCrabboxServers(ctx)
	if err != nil {
		return Server{}, SSHTarget{}, "", err
	}
	for _, server := range servers {
		if server.Labels["lease"] == id || server.Name == id {
			leaseID := server.Labels["lease"]
			target := SSHTarget{User: cfg.SSHUser, Host: server.PublicNet.IPv4.IP, Key: cfg.SSHKey, Port: cfg.SSHPort}
			useStoredTestboxKey(&target, leaseID)
			return server, target, leaseID, nil
		}
	}
	return Server{}, SSHTarget{}, "", exit(4, "lease/server not found: %s", id)
}

func (a App) stop(ctx context.Context, args []string) error {
	fs := newFlagSet("stop", a.Stderr)
	provider := fs.String("provider", defaultConfig().Provider, "provider: hetzner or aws")
	if err := parseFlags(fs, args); err != nil {
		return err
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
		if lease, err := coord.GetLease(ctx, fs.Arg(0)); err == nil {
			_, target, leaseID := leaseToServerTarget(lease, cfg)
			a.writeActionsHydrationStopBestEffort(ctx, target, leaseID)
		} else {
			fmt.Fprintf(a.Stderr, "warning: could not inspect lease before release: %v\n", err)
		}
		released, err := coord.ReleaseLease(ctx, fs.Arg(0), true)
		if err != nil {
			return err
		}
		fmt.Fprintf(a.Stderr, "released lease=%s server=%s\n", released.ID, leaseDisplayID(released))
		return nil
	}
	server, target, leaseID, err := a.findLease(ctx, cfg, fs.Arg(0))
	if err != nil {
		return err
	}
	a.writeActionsHydrationStopBestEffort(ctx, target, leaseID)
	fmt.Fprintf(a.Stderr, "deleting lease=%s server=%s name=%s\n", leaseID, server.DisplayID(), server.Name)
	return deleteServer(ctx, cfg, server)
}

func (a App) writeActionsHydrationStopBestEffort(ctx context.Context, target SSHTarget, leaseID string) {
	if leaseID == "" || target.Host == "" {
		return
	}
	stopCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := writeActionsHydrationStop(stopCtx, target, leaseID); err != nil {
		fmt.Fprintf(a.Stderr, "warning: could not stop GitHub Actions hydration for %s: %v\n", leaseID, err)
	}
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
		if err := client.DeleteServer(ctx, server.CloudID); err != nil {
			return err
		}
		if keyName := serverProviderKey(server); validCrabboxProviderKey(keyName) {
			return client.DeleteSSHKey(ctx, keyName)
		}
		return nil
	}
	client, err := newHetznerClient()
	if err != nil {
		return err
	}
	if err := client.DeleteServer(ctx, server.ID); err != nil {
		return err
	}
	if keyName := serverProviderKey(server); validCrabboxProviderKey(keyName) {
		return client.DeleteSSHKey(ctx, keyName)
	}
	return nil
}

func serverProviderKey(server Server) string {
	if server.Labels != nil && server.Labels["provider_key"] != "" {
		return server.Labels["provider_key"]
	}
	if server.Labels != nil && server.Labels["lease"] != "" {
		return providerKeyForLease(server.Labels["lease"])
	}
	return ""
}

func validCrabboxProviderKey(name string) bool {
	const prefix = "crabbox-cbx-"
	if !strings.HasPrefix(name, prefix) || len(name) != len(prefix)+12 {
		return false
	}
	for _, c := range name[len(prefix):] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func repoExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
