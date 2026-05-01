package cli

import (
	"os"
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

func TestClaimLeaseForRepoIgnoresIncompleteClaimAndRemoveIsIdempotent(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := claimLeaseForRepo("", "slug", "/repo", time.Minute, false); err != nil {
		t.Fatal(err)
	}
	if err := claimLeaseForRepo("cbx_empty", "slug", "", time.Minute, false); err != nil {
		t.Fatal(err)
	}

	path, err := leaseClaimPath("cbx_abc123abc123")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"leaseID":"cbx_abc123abc123"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := claimLeaseForRepo("cbx_abc123abc123", "blue-lobster", "/repo", 0, false); err != nil {
		t.Fatal(err)
	}
	claim, err := readLeaseClaim("cbx_abc123abc123")
	if err != nil {
		t.Fatal(err)
	}
	if claim.RepoRoot != "/repo" || claim.ClaimedAt == "" || claim.LastUsedAt == "" || claim.IdleTimeoutSeconds != 0 {
		t.Fatalf("unexpected claim: %#v", claim)
	}
	removeLeaseClaim("cbx_abc123abc123")
	removeLeaseClaim("cbx_abc123abc123")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("claim should be removed, stat err=%v", err)
	}
}

func TestReadLeaseClaimRejectsInvalidJSON(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path, err := leaseClaimPath("cbx_badbadbadbad")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = readLeaseClaim("cbx_badbadbadbad")
	if err == nil || !strings.Contains(err.Error(), "parse claim") {
		t.Fatalf("expected parse claim error, got %v", err)
	}
}
