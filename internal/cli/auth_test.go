package cli

import (
	"path/filepath"
	"testing"
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
