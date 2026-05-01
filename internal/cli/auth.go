package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

func (a App) login(ctx context.Context, args []string) error {
	fs := newFlagSet("login", a.Stderr)
	brokerURL := fs.String("url", "", "broker URL")
	provider := fs.String("provider", "", "default provider: hetzner or aws")
	tokenStdin := fs.Bool("token-stdin", false, "read broker token from stdin")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *brokerURL == "" {
		if cfg, err := loadConfig(); err == nil {
			*brokerURL = cfg.Coordinator
			if *provider == "" {
				*provider = cfg.Provider
			}
		}
	}
	if *brokerURL == "" {
		return exit(2, "login requires --url or an existing broker URL in config")
	}
	if !*tokenStdin {
		return exit(2, "login requires --token-stdin; pass secrets over stdin, not flags")
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return exit(2, "read broker token: %v", err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return exit(2, "broker token from stdin is empty")
	}
	path, cfg, err := writeBrokerLogin(*brokerURL, token, *provider)
	if err != nil {
		return err
	}
	coord, ok, err := newCoordinatorClient(cfg)
	if err != nil {
		return err
	}
	if !ok {
		return exit(2, "login wrote config but broker is not configured")
	}
	who, err := coord.Whoami(ctx)
	if err != nil {
		return err
	}
	if *jsonOut {
		return json.NewEncoder(a.Stdout).Encode(map[string]any{
			"config":   path,
			"broker":   cfg.Coordinator,
			"provider": cfg.Provider,
			"identity": who,
		})
	}
	fmt.Fprintf(a.Stdout, "logged in broker=%s provider=%s user=%s org=%s config=%s\n", cfg.Coordinator, cfg.Provider, who.Owner, who.Org, path)
	return nil
}

func (a App) logout(_ context.Context, args []string) error {
	fs := newFlagSet("logout", a.Stderr)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	path := writableConfigPath()
	if path == "" {
		return exit(2, "user config directory is unavailable")
	}
	file, err := readFileConfig(path)
	if err != nil {
		return err
	}
	if file.Broker != nil {
		file.Broker.Token = ""
	}
	file.CoordinatorToken = ""
	written, err := writeUserFileConfig(file)
	if err != nil {
		return err
	}
	if *jsonOut {
		return json.NewEncoder(a.Stdout).Encode(map[string]any{"config": written, "brokerAuth": "missing"})
	}
	fmt.Fprintf(a.Stdout, "logged out config=%s broker_auth=missing\n", written)
	return nil
}

func (a App) whoami(ctx context.Context, args []string) error {
	fs := newFlagSet("whoami", a.Stderr)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	coord, ok, err := newCoordinatorClient(cfg)
	if err != nil {
		return err
	}
	if !ok {
		return exit(2, "whoami requires a configured coordinator")
	}
	who, err := coord.Whoami(ctx)
	if err != nil {
		return err
	}
	if *jsonOut {
		return json.NewEncoder(a.Stdout).Encode(who)
	}
	fmt.Fprintf(a.Stdout, "user=%s org=%s auth=%s broker=%s\n", who.Owner, who.Org, who.Auth, cfg.Coordinator)
	return nil
}

func writeBrokerLogin(brokerURL, token, provider string) (string, Config, error) {
	path := writableConfigPath()
	if path == "" {
		return "", Config{}, exit(2, "user config directory is unavailable")
	}
	file, err := readFileConfig(path)
	if err != nil {
		return "", Config{}, err
	}
	if file.Broker == nil {
		file.Broker = &fileBrokerConfig{}
	}
	file.Broker.URL = brokerURL
	file.Broker.Token = token
	if provider != "" {
		file.Broker.Provider = provider
		file.Provider = provider
	}
	written, err := writeUserFileConfig(file)
	if err != nil {
		return "", Config{}, err
	}
	cfg, err := loadConfig()
	return written, cfg, err
}
