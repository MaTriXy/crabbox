package cli

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClaimLeaseForRepoWritesAndUpdatesClaim(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	repo := filepath.Join(t.TempDir(), "repo")
	if err := claimLeaseForRepo("cbx_123", "blue-lobster", repo, 30*time.Minute, false); err != nil {
		t.Fatal(err)
	}
	claim, err := readLeaseClaim("cbx_123")
	if err != nil {
		t.Fatal(err)
	}
	if claim.LeaseID != "cbx_123" || claim.Slug != "blue-lobster" || claim.RepoRoot != repo || claim.IdleTimeoutSeconds != 1800 {
		t.Fatalf("unexpected claim: %#v", claim)
	}
}

func TestClaimLeaseForRepoRejectsOtherRepoUnlessReclaimed(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	firstRepo := filepath.Join(t.TempDir(), "first")
	secondRepo := filepath.Join(t.TempDir(), "second")
	if err := claimLeaseForRepo("cbx_123", "blue-lobster", firstRepo, 30*time.Minute, false); err != nil {
		t.Fatal(err)
	}
	err := claimLeaseForRepo("cbx_123", "blue-lobster", secondRepo, 30*time.Minute, false)
	if err == nil || !strings.Contains(err.Error(), "use --reclaim") {
		t.Fatalf("expected reclaim error, got %v", err)
	}
	if err := claimLeaseForRepo("cbx_123", "blue-lobster", secondRepo, 30*time.Minute, true); err != nil {
		t.Fatal(err)
	}
	claim, err := readLeaseClaim("cbx_123")
	if err != nil {
		t.Fatal(err)
	}
	if claim.RepoRoot != secondRepo {
		t.Fatalf("repo root=%q want %q", claim.RepoRoot, secondRepo)
	}
}
