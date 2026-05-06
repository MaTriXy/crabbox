package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	gosdk "github.com/islo-labs/go-sdk"
	"github.com/islo-labs/go-sdk/client"
	"github.com/islo-labs/go-sdk/customauth"
	"github.com/islo-labs/go-sdk/option"
)

type isloAPI interface {
	CreateSandbox(context.Context, *gosdk.SandboxCreate) (*gosdk.SandboxResponse, error)
	GetSandbox(context.Context, string) (*gosdk.SandboxResponse, error)
	ListSandboxes(context.Context) ([]*gosdk.SandboxResponse, error)
	DeleteSandbox(context.Context, string) error
	ExecStream(context.Context, string, *gosdk.ExecRequest, io.Writer, io.Writer) (int, error)
}

type isloSDKClient struct {
	sdk        *client.Client
	auth       *customauth.Provider
	baseURL    string
	httpClient *http.Client
}

func newIsloClient(cfg Config, rt Runtime) (isloAPI, error) {
	apiKey := strings.TrimSpace(cfg.Islo.APIKey)
	if apiKey == "" {
		return nil, exit(2, "provider=islo requires ISLO_API_KEY")
	}
	baseURL := strings.TrimRight(blank(cfg.Islo.BaseURL, "https://api.islo.dev"), "/")
	httpClient := rt.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	auth := customauth.NewProvider(baseURL, apiKey, 0, httpClient)
	var baseTransport http.RoundTripper
	var timeout time.Duration
	if httpClient != nil {
		baseTransport = httpClient.Transport
		timeout = httpClient.Timeout
	}
	sdkHTTPClient := &http.Client{
		Transport: customauth.NewTransport(baseTransport, auth),
		Timeout:   timeout,
	}
	sdk := client.NewClient(option.WithBaseURL(baseURL), option.WithHTTPClient(sdkHTTPClient))
	return &isloSDKClient{sdk: sdk, auth: auth, baseURL: baseURL, httpClient: httpClient}, nil
}

func (c *isloSDKClient) CreateSandbox(ctx context.Context, req *gosdk.SandboxCreate) (*gosdk.SandboxResponse, error) {
	return c.sdk.Sandboxes.CreateSandbox(ctx, req)
}

func (c *isloSDKClient) GetSandbox(ctx context.Context, name string) (*gosdk.SandboxResponse, error) {
	return c.sdk.Sandboxes.GetSandbox(ctx, &gosdk.GetSandboxRequest{SandboxName: name})
}

func (c *isloSDKClient) ListSandboxes(ctx context.Context) ([]*gosdk.SandboxResponse, error) {
	limit := 100
	var all []*gosdk.SandboxResponse
	for offset := 0; ; offset += limit {
		page, err := c.sdk.Sandboxes.ListSandboxes(ctx, &gosdk.ListSandboxesRequest{Limit: &limit, Offset: &offset})
		if err != nil {
			return nil, err
		}
		if page == nil {
			return all, nil
		}
		items := page.GetItems()
		all = append(all, items...)
		if len(items) < limit {
			return all, nil
		}
		if total := page.GetTotal(); total > 0 && offset+len(items) >= total {
			return all, nil
		}
	}
}

func (c *isloSDKClient) DeleteSandbox(ctx context.Context, name string) error {
	_, err := c.sdk.Sandboxes.DeleteSandbox(ctx, &gosdk.DeleteSandboxRequest{SandboxName: name})
	return err
}

func (c *isloSDKClient) ExecStream(ctx context.Context, name string, req *gosdk.ExecRequest, stdout, stderr io.Writer) (int, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return 1, fmt.Errorf("encode exec request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sandboxes/"+name+"/exec/stream", bytes.NewReader(body))
	if err != nil {
		return 1, err
	}
	token, err := c.auth.Token(ctx)
	if err != nil {
		return 1, fmt.Errorf("islo auth: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 1, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return 1, fmt.Errorf("islo exec stream %s: %s", resp.Status, strings.TrimSpace(string(snippet)))
	}
	return parseIsloSSE(resp.Body, stdout, stderr)
}

func parseIsloSSE(r io.Reader, stdout, stderr io.Writer) (int, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	exitCode := 0
	event := ""
	var data []string
	flush := func() {
		if event == "" && len(data) == 0 {
			return
		}
		payload := strings.Join(data, "\n")
		switch event {
		case "stdout":
			_, _ = stdout.Write([]byte(payload))
		case "stderr":
			_, _ = stderr.Write([]byte(payload))
		case "exit":
			if n, err := strconv.Atoi(strings.TrimSpace(payload)); err == nil {
				exitCode = n
			}
		}
		event = ""
		data = data[:0]
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, found := strings.Cut(line, ":")
		if !found {
			field = line
			value = ""
		}
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			event = value
		case "data":
			data = append(data, value)
		}
	}
	flush()
	return exitCode, scanner.Err()
}
