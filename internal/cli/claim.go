package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type leaseClaim struct {
	LeaseID            string `json:"leaseID"`
	Slug               string `json:"slug,omitempty"`
	RepoRoot           string `json:"repoRoot"`
	ClaimedAt          string `json:"claimedAt"`
	LastUsedAt         string `json:"lastUsedAt"`
	IdleTimeoutSeconds int    `json:"idleTimeoutSeconds,omitempty"`
}

func claimLeaseForRepo(leaseID, slug, repoRoot string, idleTimeout time.Duration, reclaim bool) error {
	if leaseID == "" || repoRoot == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	path, err := leaseClaimPath(leaseID)
	if err != nil {
		return err
	}
	existing, err := readLeaseClaim(leaseID)
	if err != nil {
		return err
	}
	if existing.LeaseID != "" && existing.RepoRoot != "" && existing.RepoRoot != repoRoot && !reclaim {
		return exit(2, "lease %s is claimed by repo %s; use --reclaim to claim it for %s", leaseID, existing.RepoRoot, repoRoot)
	}
	if existing.ClaimedAt == "" || reclaim || existing.RepoRoot != repoRoot {
		existing.ClaimedAt = now
	}
	existing.LeaseID = leaseID
	existing.Slug = slug
	existing.RepoRoot = repoRoot
	existing.LastUsedAt = now
	if idleTimeout > 0 {
		existing.IdleTimeoutSeconds = int(idleTimeout.Seconds())
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return exit(2, "create claim directory: %v", err)
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return exit(2, "write claim %s: %v", path, err)
	}
	return nil
}

func removeLeaseClaim(leaseID string) {
	path, err := leaseClaimPath(leaseID)
	if err == nil {
		_ = os.Remove(path)
	}
}

func readLeaseClaim(leaseID string) (leaseClaim, error) {
	path, err := leaseClaimPath(leaseID)
	if err != nil {
		return leaseClaim{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return leaseClaim{}, nil
	}
	if err != nil {
		return leaseClaim{}, exit(2, "read claim %s: %v", path, err)
	}
	var claim leaseClaim
	if err := json.Unmarshal(data, &claim); err != nil {
		return leaseClaim{}, exit(2, "parse claim %s: %v", path, err)
	}
	return claim, nil
}

func leaseClaimPath(leaseID string) (string, error) {
	dir, err := crabboxStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "claims", leaseID+".json"), nil
}

func crabboxStateDir() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "crabbox"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", exit(2, "user state directory is unavailable")
	}
	return filepath.Join(dir, "crabbox", "state"), nil
}
