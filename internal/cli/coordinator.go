package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type CoordinatorClient struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

type CoordinatorLease struct {
	ID         string `json:"id"`
	Provider   string `json:"provider"`
	Profile    string `json:"profile"`
	Class      string `json:"class"`
	ServerType string `json:"serverType"`
	ServerID   int64  `json:"serverID"`
	CloudID    string `json:"cloudID"`
	ServerName string `json:"serverName"`
	Host       string `json:"host"`
	SSHUser    string `json:"sshUser"`
	SSHPort    string `json:"sshPort"`
	WorkRoot   string `json:"workRoot"`
	Keep       bool   `json:"keep"`
	State      string `json:"state"`
	ExpiresAt  string `json:"expiresAt"`
}

type CoordinatorMachine struct {
	ID         CoordinatorID     `json:"id"`
	Provider   string            `json:"provider"`
	CloudID    string            `json:"cloudID"`
	Name       string            `json:"name"`
	Status     string            `json:"status"`
	ServerType string            `json:"serverType"`
	Host       string            `json:"host"`
	Labels     map[string]string `json:"labels"`
}

type CoordinatorUsageResponse struct {
	Usage  CoordinatorUsageSummary `json:"usage"`
	Limits CoordinatorCostLimits   `json:"limits"`
}

type CoordinatorUsageSummary struct {
	Month          string                  `json:"month"`
	Scope          string                  `json:"scope"`
	Owner          string                  `json:"owner,omitempty"`
	Org            string                  `json:"org,omitempty"`
	Leases         int                     `json:"leases"`
	ActiveLeases   int                     `json:"activeLeases"`
	RuntimeSeconds int64                   `json:"runtimeSeconds"`
	EstimatedUSD   float64                 `json:"estimatedUSD"`
	ReservedUSD    float64                 `json:"reservedUSD"`
	ByOwner        []CoordinatorUsageGroup `json:"byOwner"`
	ByOrg          []CoordinatorUsageGroup `json:"byOrg"`
	ByProvider     []CoordinatorUsageGroup `json:"byProvider"`
	ByServerType   []CoordinatorUsageGroup `json:"byServerType"`
}

type CoordinatorUsageGroup struct {
	Key            string  `json:"key"`
	Leases         int     `json:"leases"`
	ActiveLeases   int     `json:"activeLeases"`
	RuntimeSeconds int64   `json:"runtimeSeconds"`
	EstimatedUSD   float64 `json:"estimatedUSD"`
	ReservedUSD    float64 `json:"reservedUSD"`
}

type CoordinatorCostLimits struct {
	MaxActiveLeases         int     `json:"maxActiveLeases"`
	MaxActiveLeasesPerOwner int     `json:"maxActiveLeasesPerOwner"`
	MaxActiveLeasesPerOrg   int     `json:"maxActiveLeasesPerOrg"`
	MaxMonthlyUSD           float64 `json:"maxMonthlyUSD"`
	MaxMonthlyUSDPerOwner   float64 `json:"maxMonthlyUSDPerOwner"`
	MaxMonthlyUSDPerOrg     float64 `json:"maxMonthlyUSDPerOrg"`
}

type CoordinatorID string

func (id *CoordinatorID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*id = CoordinatorID(s)
		return nil
	}
	var n int64
	if err := json.Unmarshal(data, &n); err == nil {
		*id = CoordinatorID(fmt.Sprint(n))
		return nil
	}
	return fmt.Errorf("invalid coordinator id: %s", string(data))
}

func newCoordinatorClient(cfg Config) (*CoordinatorClient, bool, error) {
	if cfg.Coordinator == "" {
		return nil, false, nil
	}
	base, err := url.Parse(cfg.Coordinator)
	if err != nil {
		return nil, true, exit(2, "invalid CRABBOX_COORDINATOR: %v", err)
	}
	if base.Scheme == "" || base.Host == "" {
		return nil, true, exit(2, "CRABBOX_COORDINATOR must be an absolute URL")
	}
	base.Path = strings.TrimRight(base.Path, "/")
	return &CoordinatorClient{
		BaseURL: strings.TrimRight(base.String(), "/"),
		Token:   cfg.CoordToken,
		Client: &http.Client{
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 5 * time.Minute,
				IdleConnTimeout:       90 * time.Second,
			},
		},
	}, true, nil
}

func (c *CoordinatorClient) CreateLease(ctx context.Context, cfg Config, publicKey string, keep bool, leaseID string) (CoordinatorLease, error) {
	var res struct {
		Lease CoordinatorLease `json:"lease"`
	}
	err := c.do(ctx, http.MethodPost, "/v1/leases", map[string]any{
		"leaseID":     leaseID,
		"profile":     cfg.Profile,
		"provider":    cfg.Provider,
		"class":       cfg.Class,
		"serverType":  cfg.ServerType,
		"location":    cfg.Location,
		"image":       cfg.Image,
		"awsRegion":   cfg.AWSRegion,
		"awsAMI":      cfg.AWSAMI,
		"awsSGID":     cfg.AWSSGID,
		"awsSubnetID": cfg.AWSSubnetID,
		"awsProfile":  cfg.AWSProfile,
		"awsRootGB":   cfg.AWSRootGB,
		"capacity": map[string]any{
			"market":            cfg.Capacity.Market,
			"strategy":          cfg.Capacity.Strategy,
			"fallback":          cfg.Capacity.Fallback,
			"regions":           cfg.Capacity.Regions,
			"availabilityZones": cfg.Capacity.AvailabilityZones,
		},
		"sshUser":      cfg.SSHUser,
		"sshPort":      cfg.SSHPort,
		"providerKey":  cfg.ProviderKey,
		"workRoot":     cfg.WorkRoot,
		"ttlSeconds":   int(cfg.TTL.Seconds()),
		"keep":         keep,
		"sshPublicKey": publicKey,
	}, &res)
	return res.Lease, err
}

func (c *CoordinatorClient) GetLease(ctx context.Context, id string) (CoordinatorLease, error) {
	var res struct {
		Lease CoordinatorLease `json:"lease"`
	}
	err := c.do(ctx, http.MethodGet, "/v1/leases/"+url.PathEscape(id), nil, &res)
	return res.Lease, err
}

func (c *CoordinatorClient) ReleaseLease(ctx context.Context, id string, deleteServer bool) (CoordinatorLease, error) {
	var res struct {
		Lease CoordinatorLease `json:"lease"`
	}
	err := c.do(ctx, http.MethodPost, "/v1/leases/"+url.PathEscape(id)+"/release", map[string]any{"delete": deleteServer}, &res)
	return res.Lease, err
}

func (c *CoordinatorClient) HeartbeatLease(ctx context.Context, id string) (CoordinatorLease, error) {
	var res struct {
		Lease CoordinatorLease `json:"lease"`
	}
	err := c.do(ctx, http.MethodPost, "/v1/leases/"+url.PathEscape(id)+"/heartbeat", map[string]any{}, &res)
	return res.Lease, err
}

func (c *CoordinatorClient) Pool(ctx context.Context, cfg Config) ([]CoordinatorMachine, error) {
	var res struct {
		Machines []CoordinatorMachine `json:"machines"`
	}
	path := "/v1/pool"
	if cfg.Provider != "" {
		path += "?provider=" + url.QueryEscape(cfg.Provider)
	}
	err := c.do(ctx, http.MethodGet, path, nil, &res)
	return res.Machines, err
}

func (c *CoordinatorClient) Usage(ctx context.Context, scope, owner, org, month string) (CoordinatorUsageResponse, error) {
	var res CoordinatorUsageResponse
	values := url.Values{}
	if scope != "" {
		values.Set("scope", scope)
	}
	if owner != "" {
		values.Set("owner", owner)
	}
	if org != "" {
		values.Set("org", org)
	}
	if month != "" {
		values.Set("month", month)
	}
	path := "/v1/usage"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	err := c.do(ctx, http.MethodGet, path, nil, &res)
	return res, err
}

func (c *CoordinatorClient) Health(ctx context.Context) error {
	var res map[string]any
	return c.do(ctx, http.MethodGet, "/v1/health", nil, &res)
}

func (c *CoordinatorClient) do(ctx context.Context, method, path string, body any, out any) error {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	err = c.doHTTP(ctx, method, path, data, body != nil, out)
	if err == nil || !isCoordinatorTransportError(err) {
		return err
	}
	if curlErr := c.doCurl(ctx, method, path, data, body != nil, out); curlErr == nil {
		return nil
	} else {
		return fmt.Errorf("%w; curl fallback failed: %v", err, curlErr)
	}
}

func (c *CoordinatorClient) doHTTP(ctx context.Context, method, path string, data []byte, hasBody bool, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if owner := localCoordinatorOwner(); owner != "" {
		req.Header.Set("X-Crabbox-Owner", owner)
	}
	if org := os.Getenv("CRABBOX_ORG"); org != "" {
		req.Header.Set("X-Crabbox-Org", org)
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeCoordinatorResponse(method, path, resp.StatusCode, resp.Body, out)
}

func (c *CoordinatorClient) doCurl(ctx context.Context, method, path string, data []byte, hasBody bool, out any) error {
	config, cleanup, err := c.curlConfig(method, path, data, hasBody)
	if err != nil {
		return err
	}
	defer cleanup()

	cmd := exec.CommandContext(ctx, "curl", "--config", "-")
	cmd.Stdin = strings.NewReader(config)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%v: %s", err, msg)
		}
		return err
	}
	body, status, err := splitCurlResponse(stdout.Bytes())
	if err != nil {
		return err
	}
	return decodeCoordinatorResponse(method, path, status, bytes.NewReader(body), out)
}

func (c *CoordinatorClient) curlConfig(method, path string, data []byte, hasBody bool) (string, func(), error) {
	var bodyPath string
	cleanup := func() {}
	if hasBody {
		file, err := os.CreateTemp("", "crabbox-curl-body-*")
		if err != nil {
			return "", cleanup, err
		}
		bodyPath = file.Name()
		cleanup = func() { _ = os.Remove(bodyPath) }
		if _, err := file.Write(data); err != nil {
			_ = file.Close()
			cleanup()
			return "", func() {}, err
		}
		if err := file.Close(); err != nil {
			cleanup()
			return "", func() {}, err
		}
	}

	var cfg strings.Builder
	curlConfigValue(&cfg, "url", c.BaseURL+path)
	curlConfigValue(&cfg, "request", method)
	curlConfigValue(&cfg, "connect-timeout", "10")
	curlConfigValue(&cfg, "max-time", "300")
	curlConfigFlag(&cfg, "silent")
	curlConfigFlag(&cfg, "show-error")
	curlConfigFlag(&cfg, "location")
	curlConfigValue(&cfg, "output", "-")
	curlConfigValue(&cfg, "write-out", "\n%{http_code}")
	if hasBody {
		curlConfigValue(&cfg, "header", "Content-Type: application/json")
		curlConfigValue(&cfg, "data-binary", "@"+bodyPath)
	}
	if c.Token != "" {
		curlConfigValue(&cfg, "header", "Authorization: Bearer "+c.Token)
	}
	if owner := localCoordinatorOwner(); owner != "" {
		curlConfigValue(&cfg, "header", "X-Crabbox-Owner: "+owner)
	}
	if org := os.Getenv("CRABBOX_ORG"); org != "" {
		curlConfigValue(&cfg, "header", "X-Crabbox-Org: "+org)
	}
	return cfg.String(), cleanup, nil
}

func curlConfigValue(out *strings.Builder, key, value string) {
	fmt.Fprintf(out, "%s = %s\n", key, strconv.Quote(value))
}

func curlConfigFlag(out *strings.Builder, key string) {
	fmt.Fprintln(out, key)
}

func splitCurlResponse(data []byte) ([]byte, int, error) {
	idx := bytes.LastIndexByte(data, '\n')
	if idx < 0 || idx+1 >= len(data) {
		return nil, 0, fmt.Errorf("curl response missing status")
	}
	status, err := strconv.Atoi(strings.TrimSpace(string(data[idx+1:])))
	if err != nil {
		return nil, 0, fmt.Errorf("curl response invalid status: %w", err)
	}
	return data[:idx], status, nil
}

func decodeCoordinatorResponse(method, path string, statusCode int, body io.Reader, out any) error {
	if statusCode < 200 || statusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(body, 600))
		msg := strings.TrimSpace(string(data))
		if msg != "" {
			return fmt.Errorf("coordinator %s %s: http %d: %s", method, path, statusCode, msg)
		}
		return fmt.Errorf("coordinator %s %s: http %d", method, path, statusCode)
	}
	if out != nil {
		if err := json.NewDecoder(body).Decode(out); err != nil {
			return err
		}
	}
	return nil
}

func isCoordinatorTransportError(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr)
}

func localCoordinatorOwner() string {
	for _, key := range []string{"CRABBOX_OWNER", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	out, err := exec.Command("git", "config", "--get", "user.email").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func leaseToServerTarget(lease CoordinatorLease, cfg Config) (Server, SSHTarget, string) {
	server := Server{
		Provider: lease.Provider,
		CloudID:  lease.CloudID,
		ID:       lease.ServerID,
		Name:     lease.ServerName,
		Status:   lease.State,
		Labels: map[string]string{
			"lease": lease.ID,
			"keep":  fmt.Sprint(lease.Keep),
		},
	}
	if server.Provider == "" {
		server.Provider = cfg.Provider
	}
	server.PublicNet.IPv4.IP = lease.Host
	server.ServerType.Name = lease.ServerType
	target := SSHTarget{User: lease.SSHUser, Host: lease.Host, Key: cfg.SSHKey, Port: lease.SSHPort}
	useStoredTestboxKey(&target, lease.ID)
	return server, target, lease.ID
}
