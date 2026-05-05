package cli

import (
	"strings"
	"testing"
)

func TestWebCodeURLs(t *testing.T) {
	if got := webCodeAgentURL("https://crabbox.openclaw.ai", "cbx_abcdef123456", "code_abc"); got != "wss://crabbox.openclaw.ai/v1/leases/cbx_abcdef123456/code/agent?ticket=code_abc" {
		t.Fatalf("agent URL=%q", got)
	}
	if got := webCodePortalURL("https://crabbox.openclaw.ai/", "cbx_abcdef123456"); got != "https://crabbox.openclaw.ai/portal/leases/cbx_abcdef123456/code/" {
		t.Fatalf("portal URL=%q", got)
	}
}

func TestCodeUpstreamPathStripsPortalLeasePrefix(t *testing.T) {
	tests := map[string]string{
		"/portal/leases/cbx_abcdef123456/code/":                    "/",
		"/portal/leases/cbx_abcdef123456/code/static/main.js":      "/static/main.js",
		"/portal/leases/cbx_abcdef123456/code/?folder=/work/repo":  "/?folder=/work/repo",
		"/portal/leases/blue-lobster/code/vscode-remote-resource":  "/vscode-remote-resource",
		"/portal/leases/blue-lobster/vnc/viewer":                   "/portal/leases/blue-lobster/vnc/viewer",
		"/portal/leases/blue-lobster/code/proxy/3000/?q=hello+you": "/proxy/3000/?q=hello+you",
	}
	for input, want := range tests {
		if got := codeUpstreamPath(input); got != want {
			t.Fatalf("codeUpstreamPath(%q)=%q want %q", input, got, want)
		}
	}
}

func TestStartCodeServerCommand(t *testing.T) {
	got := startCodeServerCommand("/work/crabbox/cbx_abcdef123456/repo")
	for _, want := range []string{
		"/usr/local/bin/code-server",
		"--auth none",
		"--bind-addr 127.0.0.1:8080",
		"VSCODE_PROXY_URI='./proxy/{{port}}'",
		"/tmp/crabbox-code-server.log",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("startCodeServerCommand missing %q:\n%s", want, got)
		}
	}
}
