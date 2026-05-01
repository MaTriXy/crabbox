package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

func (a App) config(_ context.Context, args []string) error {
	if len(args) == 0 {
		return exit(2, "usage: crabbox config show|path|set-broker")
	}
	switch args[0] {
	case "path":
		path := userConfigPath()
		if path == "" {
			return exit(2, "user config directory is unavailable")
		}
		fmt.Fprintln(a.Stdout, path)
		return nil
	case "show":
		return a.configShow(args[1:])
	case "set-broker":
		return a.configSetBroker(args[1:])
	default:
		return exit(2, "unknown config command %q", args[0])
	}
}

func (a App) configShow(args []string) error {
	fs := newFlagSet("config show", a.Stderr)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	view := map[string]any{
		"profile":     cfg.Profile,
		"provider":    cfg.Provider,
		"class":       cfg.Class,
		"serverType":  cfg.ServerType,
		"coordinator": cfg.Coordinator,
		"brokerAuth":  tokenState(cfg.CoordToken),
		"sshKey":      cfg.SSHKey,
		"sshUser":     cfg.SSHUser,
		"sshPort":     cfg.SSHPort,
		"workRoot":    cfg.WorkRoot,
		"hetzner": map[string]any{
			"location": cfg.Location,
			"image":    cfg.Image,
			"sshKey":   cfg.ProviderKey,
		},
		"aws": map[string]any{
			"region":          cfg.AWSRegion,
			"ami":             cfg.AWSAMI,
			"securityGroupId": cfg.AWSSGID,
			"subnetId":        cfg.AWSSubnetID,
			"instanceProfile": cfg.AWSProfile,
			"rootGB":          cfg.AWSRootGB,
		},
	}
	if *jsonOut {
		return json.NewEncoder(a.Stdout).Encode(view)
	}
	fmt.Fprintf(a.Stdout, "config=%s\n", userConfigPath())
	fmt.Fprintf(a.Stdout, "provider=%s class=%s type=%s profile=%s\n", cfg.Provider, cfg.Class, cfg.ServerType, cfg.Profile)
	fmt.Fprintf(a.Stdout, "broker=%s auth=%s\n", blank(cfg.Coordinator, "-"), tokenState(cfg.CoordToken))
	fmt.Fprintf(a.Stdout, "ssh=%s@<host>:%s key=%s\n", cfg.SSHUser, cfg.SSHPort, cfg.SSHKey)
	fmt.Fprintf(a.Stdout, "aws region=%s root_gb=%d\n", cfg.AWSRegion, cfg.AWSRootGB)
	return nil
}

func (a App) configSetBroker(args []string) error {
	fs := newFlagSet("config set-broker", a.Stderr)
	url := fs.String("url", "", "broker URL")
	provider := fs.String("provider", "", "default provider: hetzner or aws")
	tokenStdin := fs.Bool("token-stdin", false, "read broker token from stdin")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *url == "" {
		return exit(2, "config set-broker requires --url")
	}
	var token string
	if *tokenStdin {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return exit(2, "read broker token: %v", err)
		}
		token = strings.TrimSpace(string(data))
		if token == "" {
			return exit(2, "broker token from stdin is empty")
		}
	}
	path := userConfigPath()
	if path == "" {
		return exit(2, "user config directory is unavailable")
	}
	file, err := readFileConfig(path)
	if err != nil {
		return err
	}
	if file.Broker == nil {
		file.Broker = &fileBrokerConfig{}
	}
	file.Broker.URL = *url
	if token != "" {
		file.Broker.Token = token
	}
	if *provider != "" {
		file.Broker.Provider = *provider
		file.Provider = *provider
	}
	written, err := writeUserFileConfig(file)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "wrote %s broker=%s auth=%s\n", written, *url, tokenState(file.Broker.Token))
	return nil
}

func tokenState(token string) string {
	if token == "" {
		return "missing"
	}
	return "configured"
}

func blank(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
