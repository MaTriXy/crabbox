package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

type noProviderFlags struct{}

func NoProviderFlags() any { return noProviderFlags{} }

func NewHetznerLeaseBackend(spec ProviderSpec, cfg Config, rt Runtime) Backend {
	cfg.Provider = "hetzner"
	return &hetznerLeaseBackend{directSSHBackend: directSSHBackend{spec: spec, cfg: cfg, rt: rt}}
}

func NewAWSLeaseBackend(spec ProviderSpec, cfg Config, rt Runtime) Backend {
	cfg.Provider = "aws"
	return &awsLeaseBackend{directSSHBackend: directSSHBackend{spec: spec, cfg: cfg, rt: rt}}
}

func NewStaticSSHLeaseBackend(spec ProviderSpec, cfg Config, rt Runtime) Backend {
	cfg.Provider = staticProvider
	return &staticLeaseBackend{directSSHBackend: directSSHBackend{spec: spec, cfg: cfg, rt: rt}}
}

type directSSHBackend struct {
	spec ProviderSpec
	cfg  Config
	rt   Runtime
}

func (b *directSSHBackend) Spec() ProviderSpec { return b.spec }

func (b *directSSHBackend) cleanupServers(ctx context.Context, req CleanupRequest, servers []Server) error {
	_ = ctx
	_ = req
	for _, s := range servers {
		shouldDelete, reason := shouldCleanupServer(s, time.Now().UTC())
		if !shouldDelete {
			fmt.Fprintf(b.rt.Stderr, "skip server id=%s name=%s reason=%s\n", s.DisplayID(), s.Name, reason)
			continue
		}
		fmt.Fprintf(b.rt.Stderr, "delete server id=%s name=%s\n", s.DisplayID(), s.Name)
		if !req.DryRun {
			if err := deleteServer(ctx, b.cfg, s); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *directSSHBackend) touch(ctx context.Context, server Server, state string) Server {
	return touchDirectLeaseBestEffort(ctx, b.cfg, server, state, b.rt.Stderr)
}

func touchDirectLeaseBestEffort(ctx context.Context, cfg Config, server Server, state string, stderr io.Writer) Server {
	if server.Labels == nil {
		server.Labels = map[string]string{}
	}
	server.Labels = touchDirectLeaseLabels(server.Labels, cfg, state, time.Now().UTC())
	if isStaticProvider(cfg.Provider) || server.Provider == staticProvider {
		return server
	}
	if cfg.Provider == "aws" || server.Provider == "aws" || strings.HasPrefix(server.CloudID, "i-") {
		client, err := newAWSClient(ctx, cfg)
		if err != nil {
			fmt.Fprintf(stderr, "warning: direct touch state=%s: %v\n", state, err)
			return server
		}
		if err := client.SetTags(ctx, server.CloudID, server.Labels); err != nil {
			fmt.Fprintf(stderr, "warning: direct touch state=%s: %v\n", state, err)
		}
		return server
	}
	client, err := newHetznerClient()
	if err != nil {
		fmt.Fprintf(stderr, "warning: direct touch state=%s: %v\n", state, err)
		return server
	}
	if err := client.SetLabels(ctx, server.ID, server.Labels); err != nil {
		fmt.Fprintf(stderr, "warning: direct touch state=%s: %v\n", state, err)
	}
	return server
}

func chooseAWSRegion(ctx context.Context, cfg Config, stderr io.Writer) Config {
	if cfg.Provider != "aws" || cfg.Capacity.Market != "spot" || len(cfg.Capacity.Regions) < 2 {
		return cfg
	}
	client, err := newAWSClient(ctx, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "warning: spot placement score unavailable: %v\n", err)
		return cfg
	}
	scores, err := client.SpotPlacementScores(ctx, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "warning: spot placement score unavailable: %v\n", err)
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
		fmt.Fprintf(stderr, "selected aws region=%s spot_score=%d previous=%s\n", best, score, cfg.AWSRegion)
		cfg.AWSRegion = best
	}
	return cfg
}

func acquireAttemptsRetry(rt Runtime, keep bool, acquire func() (LeaseTarget, error)) (LeaseTarget, error) {
	var lastErr error
	attempts := acquireAttempts(keep)
	for attempt := 1; attempt <= attempts; attempt++ {
		lease, err := acquire()
		if err == nil {
			return lease, nil
		}
		lastErr = err
		if attempt == attempts || !isBootstrapWaitError(err) {
			return LeaseTarget{}, err
		}
		fmt.Fprintf(rt.Stderr, "warning: bootstrap failed; retrying with fresh lease: %v\n", err)
	}
	return LeaseTarget{}, lastErr
}
