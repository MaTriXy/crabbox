package cli

import (
	"context"
	"fmt"
	"time"
)

type staticLeaseBackend struct{ directSSHBackend }

func (b *staticLeaseBackend) Acquire(ctx context.Context, req AcquireRequest) (LeaseTarget, error) {
	server, target, leaseID, err := staticLease(b.cfg)
	if err != nil {
		return LeaseTarget{}, err
	}
	fmt.Fprintf(b.rt.Stderr, "using static target lease=%s slug=%s target=%s windows_mode=%s host=%s keep=%v\n", leaseID, serverSlug(server), b.cfg.TargetOS, b.cfg.WindowsMode, target.Host, req.Keep)
	if err := waitForSSH(ctx, &target, b.rt.Stderr); err != nil {
		return LeaseTarget{}, err
	}
	server.Labels["state"] = "ready"
	return LeaseTarget{Server: server, SSH: target, LeaseID: leaseID}, nil
}

func (b *staticLeaseBackend) Resolve(_ context.Context, req ResolveRequest) (LeaseTarget, error) {
	server, target, leaseID, err := staticLease(b.cfg)
	if err != nil {
		return LeaseTarget{}, err
	}
	if req.ID == "" || req.ID == leaseID || req.ID == server.Name || req.ID == serverSlug(server) || req.ID == b.cfg.Static.Host {
		return LeaseTarget{Server: server, SSH: target, LeaseID: leaseID}, nil
	}
	return LeaseTarget{}, exit(4, "static lease not found: %s", req.ID)
}

func (b *staticLeaseBackend) List(_ context.Context, req ListRequest) ([]LeaseView, error) {
	_ = req
	server, _, _, err := staticLease(b.cfg)
	if err != nil {
		return nil, err
	}
	return []LeaseView{server}, nil
}

func (b *staticLeaseBackend) ReleaseLease(_ context.Context, req ReleaseLeaseRequest) error {
	removeLeaseClaim(req.Lease.LeaseID)
	return nil
}

func (b *staticLeaseBackend) Touch(_ context.Context, req TouchRequest) (Server, error) {
	server := req.Lease.Server
	if server.Labels == nil {
		server.Labels = map[string]string{}
	}
	server.Labels = touchDirectLeaseLabels(server.Labels, b.cfg, req.State, time.Now().UTC())
	return server, nil
}

func (b *staticLeaseBackend) Cleanup(context.Context, CleanupRequest) error {
	return exit(2, "machine cleanup is not supported for provider=%s", b.cfg.Provider)
}
