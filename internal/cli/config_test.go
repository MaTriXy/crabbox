package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFromUserFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("CRABBOX_CONFIG", "")
	t.Setenv("CRABBOX_PROVIDER", "")
	t.Setenv("CRABBOX_DEFAULT_CLASS", "")
	path := userConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{
  "broker": {
    "url": "https://crabbox.example.test",
    "token": "secret",
    "provider": "aws"
  },
  "class": "standard",
  "aws": {
    "region": "eu-west-1",
    "rootGB": 800
  },
  "ssh": {
    "key": "~/.ssh/crabbox"
  }
}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "aws" {
		t.Fatalf("Provider=%q want aws", cfg.Provider)
	}
	if cfg.ServerType != "c7a.8xlarge" {
		t.Fatalf("ServerType=%q want c7a.8xlarge", cfg.ServerType)
	}
	if cfg.Coordinator != "https://crabbox.example.test" || cfg.CoordToken != "secret" {
		t.Fatalf("broker config not loaded: %#v", cfg)
	}
	if cfg.AWSRootGB != 800 {
		t.Fatalf("AWSRootGB=%d want 800", cfg.AWSRootGB)
	}
	if cfg.SSHKey != filepath.Join(home, ".ssh", "crabbox") {
		t.Fatalf("SSHKey=%q", cfg.SSHKey)
	}
}

func TestEnvOverridesConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("CRABBOX_CONFIG", "")
	t.Setenv("CRABBOX_PROVIDER", "hetzner")
	t.Setenv("CRABBOX_DEFAULT_CLASS", "fast")
	path := userConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"provider":"aws","class":"beast"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "hetzner" || cfg.Class != "fast" || cfg.ServerType != "ccx43" {
		t.Fatalf("unexpected config: provider=%s class=%s type=%s", cfg.Provider, cfg.Class, cfg.ServerType)
	}
}
