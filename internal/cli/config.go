package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Profile     string
	Provider    string
	Class       string
	ServerType  string
	Coordinator string
	CoordToken  string
	Location    string
	Image       string
	AWSRegion   string
	AWSAMI      string
	AWSSGID     string
	AWSSubnetID string
	AWSProfile  string
	AWSRootGB   int32
	SSHUser     string
	SSHKey      string
	SSHPort     string
	ProviderKey string
	WorkRoot    string
	TTL         time.Duration
}

func defaultConfig() Config {
	cfg, err := loadConfig()
	if err != nil {
		return baseConfig()
	}
	return cfg
}

func loadConfig() (Config, error) {
	cfg := baseConfig()
	for _, path := range configPaths() {
		if err := applyConfigFile(&cfg, path); err != nil {
			return Config{}, err
		}
	}
	applyEnv(&cfg)
	if cfg.ServerType == "" {
		cfg.ServerType = serverTypeForProviderClass(cfg.Provider, cfg.Class)
	}
	return cfg, nil
}

func baseConfig() Config {
	home, _ := os.UserHomeDir()
	sshKey := ""
	if home != "" {
		sshKey = filepath.Join(home, ".ssh", "id_ed25519")
	}

	class := "beast"
	provider := "hetzner"
	return Config{
		Profile:     "openclaw-check",
		Provider:    provider,
		Class:       class,
		ServerType:  "",
		Location:    "fsn1",
		Image:       "ubuntu-24.04",
		AWSRegion:   "eu-west-1",
		AWSRootGB:   400,
		SSHUser:     "crabbox",
		SSHKey:      sshKey,
		SSHPort:     "2222",
		ProviderKey: "crabbox-steipete",
		WorkRoot:    "/work/crabbox",
		TTL:         90 * time.Minute,
	}
}

type fileConfig struct {
	Profile          string             `json:"profile,omitempty"`
	Provider         string             `json:"provider,omitempty"`
	Class            string             `json:"class,omitempty"`
	ServerType       string             `json:"serverType,omitempty"`
	Coordinator      string             `json:"coordinator,omitempty"`
	CoordinatorToken string             `json:"coordinatorToken,omitempty"`
	Broker           *fileBrokerConfig  `json:"broker,omitempty"`
	Hetzner          *fileHetznerConfig `json:"hetzner,omitempty"`
	AWS              *fileAWSConfig     `json:"aws,omitempty"`
	SSH              *fileSSHConfig     `json:"ssh,omitempty"`
	WorkRoot         string             `json:"workRoot,omitempty"`
}

type fileBrokerConfig struct {
	URL      string `json:"url,omitempty"`
	Token    string `json:"token,omitempty"`
	Provider string `json:"provider,omitempty"`
}

type fileHetznerConfig struct {
	Location string `json:"location,omitempty"`
	Image    string `json:"image,omitempty"`
	SSHKey   string `json:"sshKey,omitempty"`
}

type fileAWSConfig struct {
	Region          string `json:"region,omitempty"`
	AMI             string `json:"ami,omitempty"`
	SecurityGroupID string `json:"securityGroupId,omitempty"`
	SubnetID        string `json:"subnetId,omitempty"`
	InstanceProfile string `json:"instanceProfile,omitempty"`
	RootGB          int32  `json:"rootGB,omitempty"`
}

type fileSSHConfig struct {
	User string `json:"user,omitempty"`
	Key  string `json:"key,omitempty"`
	Port string `json:"port,omitempty"`
}

func configPaths() []string {
	if explicit := os.Getenv("CRABBOX_CONFIG"); explicit != "" {
		return []string{explicit}
	}
	paths := make([]string, 0, 3)
	if userPath := userConfigPath(); userPath != "" {
		paths = append(paths, userPath)
	}
	for _, path := range []string{"crabbox.json", ".crabbox.json"} {
		if _, err := os.Stat(path); err == nil {
			paths = append(paths, path)
		}
	}
	return paths
}

func userConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "crabbox", "config.json")
}

func readFileConfig(path string) (fileConfig, error) {
	var cfg fileConfig
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, exit(2, "read config %s: %v", path, err)
	}
	if len(data) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, exit(2, "parse config %s: %v", path, err)
	}
	return cfg, nil
}

func writeUserFileConfig(cfg fileConfig) (string, error) {
	path := userConfigPath()
	if path == "" {
		return "", exit(2, "user config directory is unavailable")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", exit(2, "create config directory: %v", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", exit(2, "write config %s: %v", path, err)
	}
	return path, nil
}

func applyConfigFile(cfg *Config, path string) error {
	file, err := readFileConfig(path)
	if err != nil {
		return err
	}
	applyFileConfig(cfg, file)
	return nil
}

func applyFileConfig(cfg *Config, file fileConfig) {
	if file.Profile != "" {
		cfg.Profile = file.Profile
	}
	if file.Provider != "" {
		cfg.Provider = file.Provider
	}
	if file.Class != "" {
		cfg.Class = file.Class
	}
	if file.ServerType != "" {
		cfg.ServerType = file.ServerType
	}
	if file.Coordinator != "" {
		cfg.Coordinator = file.Coordinator
	}
	if file.CoordinatorToken != "" {
		cfg.CoordToken = file.CoordinatorToken
	}
	if file.Broker != nil {
		if file.Broker.URL != "" {
			cfg.Coordinator = file.Broker.URL
		}
		if file.Broker.Token != "" {
			cfg.CoordToken = file.Broker.Token
		}
		if file.Broker.Provider != "" {
			cfg.Provider = file.Broker.Provider
		}
	}
	if file.Hetzner != nil {
		if file.Hetzner.Location != "" {
			cfg.Location = file.Hetzner.Location
		}
		if file.Hetzner.Image != "" {
			cfg.Image = file.Hetzner.Image
		}
		if file.Hetzner.SSHKey != "" {
			cfg.ProviderKey = file.Hetzner.SSHKey
		}
	}
	if file.AWS != nil {
		if file.AWS.Region != "" {
			cfg.AWSRegion = file.AWS.Region
		}
		if file.AWS.AMI != "" {
			cfg.AWSAMI = file.AWS.AMI
		}
		if file.AWS.SecurityGroupID != "" {
			cfg.AWSSGID = file.AWS.SecurityGroupID
		}
		if file.AWS.SubnetID != "" {
			cfg.AWSSubnetID = file.AWS.SubnetID
		}
		if file.AWS.InstanceProfile != "" {
			cfg.AWSProfile = file.AWS.InstanceProfile
		}
		if file.AWS.RootGB > 0 {
			cfg.AWSRootGB = file.AWS.RootGB
		}
	}
	if file.SSH != nil {
		if file.SSH.User != "" {
			cfg.SSHUser = file.SSH.User
		}
		if file.SSH.Key != "" {
			cfg.SSHKey = expandUserPath(file.SSH.Key)
		}
		if file.SSH.Port != "" {
			cfg.SSHPort = file.SSH.Port
		}
	}
	if file.WorkRoot != "" {
		cfg.WorkRoot = file.WorkRoot
	}
}

func applyEnv(cfg *Config) {
	cfg.Profile = getenv("CRABBOX_PROFILE", cfg.Profile)
	cfg.Provider = getenv("CRABBOX_PROVIDER", cfg.Provider)
	cfg.Class = getenv("CRABBOX_DEFAULT_CLASS", cfg.Class)
	cfg.ServerType = getenv("CRABBOX_SERVER_TYPE", cfg.ServerType)
	cfg.Coordinator = getenv("CRABBOX_COORDINATOR", cfg.Coordinator)
	cfg.CoordToken = getenv("CRABBOX_COORDINATOR_TOKEN", cfg.CoordToken)
	cfg.Location = getenv("CRABBOX_HETZNER_LOCATION", cfg.Location)
	cfg.Image = getenv("CRABBOX_HETZNER_IMAGE", cfg.Image)
	cfg.AWSRegion = getenv("CRABBOX_AWS_REGION", getenv("AWS_REGION", cfg.AWSRegion))
	cfg.AWSAMI = getenv("CRABBOX_AWS_AMI", cfg.AWSAMI)
	cfg.AWSSGID = getenv("CRABBOX_AWS_SECURITY_GROUP_ID", cfg.AWSSGID)
	cfg.AWSSubnetID = getenv("CRABBOX_AWS_SUBNET_ID", cfg.AWSSubnetID)
	cfg.AWSProfile = getenv("CRABBOX_AWS_INSTANCE_PROFILE", cfg.AWSProfile)
	cfg.AWSRootGB = int32(getenvInt("CRABBOX_AWS_ROOT_GB", int(cfg.AWSRootGB)))
	cfg.SSHUser = getenv("CRABBOX_SSH_USER", cfg.SSHUser)
	cfg.SSHKey = getenv("CRABBOX_SSH_KEY", cfg.SSHKey)
	cfg.SSHPort = getenv("CRABBOX_SSH_PORT", cfg.SSHPort)
	cfg.ProviderKey = getenv("CRABBOX_HETZNER_SSH_KEY", cfg.ProviderKey)
	cfg.WorkRoot = getenv("CRABBOX_WORK_ROOT", cfg.WorkRoot)
}

func expandUserPath(path string) string {
	if path == "~" {
		home, _ := os.UserHomeDir()
		if home != "" {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		if home != "" {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func serverTypeForClass(class string) string {
	return serverTypeCandidatesForClass(class)[0]
}

func serverTypeForProviderClass(provider, class string) string {
	if provider == "aws" {
		return awsInstanceTypeCandidatesForClass(class)[0]
	}
	return serverTypeForClass(class)
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

func awsInstanceTypeCandidatesForClass(class string) []string {
	switch class {
	case "standard":
		return []string{"c7a.8xlarge", "c7a.4xlarge"}
	case "fast":
		return []string{"c7a.16xlarge", "c7a.12xlarge", "c7a.8xlarge"}
	case "large":
		return []string{"c7a.24xlarge", "c7a.16xlarge", "c7a.12xlarge"}
	case "beast":
		return []string{"c7a.48xlarge", "c7a.32xlarge", "c7a.24xlarge", "c7a.16xlarge"}
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

func getenvInt(name string, fallback int) int {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
