package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteBrokerLoginStoresTokenInUserConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("CRABBOX_CONFIG", "")
	t.Setenv("CRABBOX_COORDINATOR", "")
	t.Setenv("CRABBOX_COORDINATOR_TOKEN", "")
	t.Setenv("CRABBOX_PROVIDER", "")

	path, cfg, err := writeBrokerLogin("https://crabbox.example.test", "secret", "aws")
	if err != nil {
		t.Fatal(err)
	}
	if path != userConfigPath() {
		t.Fatalf("path=%q want %q", path, userConfigPath())
	}
	if cfg.Coordinator != "https://crabbox.example.test" || cfg.CoordToken != "secret" || cfg.Provider != "aws" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestWriteBrokerLoginHonorsExplicitConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	explicit := filepath.Join(home, "isolated.yaml")
	t.Setenv("CRABBOX_CONFIG", explicit)
	t.Setenv("CRABBOX_COORDINATOR", "")
	t.Setenv("CRABBOX_COORDINATOR_TOKEN", "")
	t.Setenv("CRABBOX_PROVIDER", "")

	path, cfg, err := writeBrokerLogin("https://crabbox.example.test", "secret", "aws")
	if err != nil {
		t.Fatal(err)
	}
	if path != explicit {
		t.Fatalf("path=%q want %q", path, explicit)
	}
	if cfg.Coordinator != "https://crabbox.example.test" || cfg.CoordToken != "secret" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestGitHubLoginNoBrowserStoresReturnedToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("CRABBOX_CONFIG", "")
	t.Setenv("CRABBOX_COORDINATOR", "")
	t.Setenv("CRABBOX_COORDINATOR_TOKEN", "")
	t.Setenv("CRABBOX_PROVIDER", "")

	var seenPollSecretHash string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/auth/github/start":
			var body struct {
				PollSecretHash string `json:"pollSecretHash"`
				Provider       string `json:"provider"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Provider != "aws" {
				t.Fatalf("provider=%q", body.Provider)
			}
			if len(body.PollSecretHash) != 64 {
				t.Fatalf("poll secret hash=%q", body.PollSecretHash)
			}
			seenPollSecretHash = body.PollSecretHash
			_ = json.NewEncoder(w).Encode(CoordinatorGitHubLoginStart{
				LoginID:   "login_test",
				URL:       "https://github.com/login/oauth/authorize?state=test",
				ExpiresAt: time.Now().Add(time.Minute).Format(time.RFC3339),
			})
		case "/v1/auth/github/poll":
			var body struct {
				LoginID    string `json:"loginID"`
				PollSecret string `json:"pollSecret"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.LoginID != "login_test" {
				t.Fatalf("loginID=%q", body.LoginID)
			}
			if sha256Hex(body.PollSecret) != seenPollSecretHash {
				t.Fatal("poll secret did not match start hash")
			}
			_ = json.NewEncoder(w).Encode(CoordinatorGitHubLoginPoll{
				Status:   "complete",
				Token:    "github-session-token",
				Owner:    "friend@example.com",
				Org:      "openclaw",
				Login:    "friend",
				Provider: "aws",
			})
		case "/v1/whoami":
			if got := r.Header.Get("Authorization"); got != "Bearer github-session-token" {
				t.Fatalf("authorization=%q", got)
			}
			_ = json.NewEncoder(w).Encode(CoordinatorWhoami{
				Owner: "friend@example.com",
				Org:   "openclaw",
				Auth:  "github",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr}
	if err := app.login(context.Background(), []string{"--url", server.URL, "--provider", "aws", "--no-browser"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "open this GitHub login URL") {
		t.Fatalf("stderr=%q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "user=friend@example.com") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Coordinator != server.URL || cfg.CoordToken != "github-session-token" || cfg.Provider != "aws" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}
