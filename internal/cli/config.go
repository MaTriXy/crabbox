package cli

import (
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	Profile     string
	Class       string
	ServerType  string
	Location    string
	Image       string
	SSHUser     string
	SSHKey      string
	SSHPort     string
	ProviderKey string
	WorkRoot    string
	TTL         time.Duration
}

func defaultConfig() Config {
	home, _ := os.UserHomeDir()
	sshKey := os.Getenv("CRABBOX_SSH_KEY")
	if sshKey == "" && home != "" {
		sshKey = filepath.Join(home, ".ssh", "id_ed25519")
	}

	class := getenv("CRABBOX_DEFAULT_CLASS", "beast")
	return Config{
		Profile:     getenv("CRABBOX_PROFILE", "openclaw-check"),
		Class:       class,
		ServerType:  serverTypeForClass(class),
		Location:    getenv("CRABBOX_HETZNER_LOCATION", "fsn1"),
		Image:       getenv("CRABBOX_HETZNER_IMAGE", "ubuntu-24.04"),
		SSHUser:     getenv("CRABBOX_SSH_USER", "crabbox"),
		SSHKey:      sshKey,
		SSHPort:     getenv("CRABBOX_SSH_PORT", "2222"),
		ProviderKey: getenv("CRABBOX_HETZNER_SSH_KEY", "crabbox-steipete"),
		WorkRoot:    getenv("CRABBOX_WORK_ROOT", "/work/crabbox"),
		TTL:         90 * time.Minute,
	}
}

func serverTypeForClass(class string) string {
	return serverTypeCandidatesForClass(class)[0]
}

func serverTypeCandidatesForClass(class string) []string {
	switch class {
	case "standard":
		return []string{"ccx33", "cpx62", "cx53"}
	case "fast":
		return []string{"ccx43", "cpx62", "cx53"}
	case "large":
		return []string{"ccx53", "ccx43", "cpx62", "cx53"}
	case "beast":
		return []string{"ccx63", "ccx53", "ccx43", "cpx62", "cx53"}
	default:
		return []string{class}
	}
}

func getenv(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
