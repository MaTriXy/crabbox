package islo

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestParseIsloSSE(t *testing.T) {
	body := strings.Join([]string{
		"event: stdout",
		"data: hello",
		"",
		"event: stderr",
		"data: warn",
		"",
		"event: exit",
		"data: 7",
		"",
	}, "\n")
	var stdout, stderr bytes.Buffer
	code, err := parseIsloSSE(strings.NewReader(body), &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if code != 7 || stdout.String() != "hello" || stderr.String() != "warn" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestParseIsloSSERequiresExitEvent(t *testing.T) {
	body := strings.Join([]string{
		"event: stdout",
		"data: partial",
		"",
	}, "\n")
	var stdout, stderr bytes.Buffer
	code, err := parseIsloSSE(strings.NewReader(body), &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "without exit event") {
		t.Fatalf("code=%d err=%v, want missing exit event error", code, err)
	}
	if stdout.String() != "partial" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestParseIsloSSERejectsInvalidExitEvent(t *testing.T) {
	body := strings.Join([]string{
		"event: exit",
		"data: nope",
		"",
	}, "\n")
	if _, err := parseIsloSSE(strings.NewReader(body), &bytes.Buffer{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "invalid exit event") {
		t.Fatalf("err=%v, want invalid exit event error", err)
	}
}

func TestIsloExecCommandPreservesShellString(t *testing.T) {
	got, err := isloExecCommand([]string{"pnpm install && pnpm test"}, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"bash", "-lc", "pnpm install && pnpm test"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("command=%#v want %#v", got, want)
	}
}

func TestIsloExecCommandQuotesImplicitShellArgv(t *testing.T) {
	got, err := isloExecCommand([]string{"FOO=bar", "pnpm", "test"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != "bash" || got[1] != "-lc" || !strings.Contains(got[2], "FOO=") || !strings.Contains(got[2], "'pnpm'") {
		t.Fatalf("command=%#v", got)
	}
}

func TestLeadingEnvAssignmentUsesShell(t *testing.T) {
	if !leadingEnvAssignment([]string{"FOO=bar", "pnpm", "test"}) {
		t.Fatal("expected leading env assignment to require shell")
	}
	if leadingEnvAssignment([]string{"pnpm", "test"}) {
		t.Fatal("plain argv should not require shell")
	}
}

func TestIsloStatusReady(t *testing.T) {
	for _, status := range []string{"ready", "running", "started", "active"} {
		if !isloStatusReady(status) {
			t.Fatalf("expected %q ready", status)
		}
	}
	if isloStatusReady("stopped") {
		t.Fatal("stopped should not be ready")
	}
}

func TestResolveIsloLeaseIDRejectsUnclaimedRawSandbox(t *testing.T) {
	if _, _, err := resolveIsloLeaseID("production", "", false); err == nil {
		t.Fatal("expected raw non-Crabbox sandbox to be rejected")
	}
	leaseID, name, err := resolveIsloLeaseID("crabbox-repo-abcdef", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if leaseID != "isb_crabbox-repo-abcdef" || name != "crabbox-repo-abcdef" {
		t.Fatalf("lease=%q name=%q", leaseID, name)
	}
}

func TestNewIsloSandboxNameUsesCrabboxPrefix(t *testing.T) {
	name := newIsloSandboxName(Repo{Name: "repo"})
	if !strings.HasPrefix(name, "crabbox-repo-") {
		t.Fatalf("name=%q", name)
	}
	if !isCrabboxIsloSandboxName(name) {
		t.Fatalf("expected %q to be recognized as Crabbox-owned", name)
	}
}

func TestIsloSDKClientListUsesInjectedHTTPAndPaginates(t *testing.T) {
	authHits := 0
	listHits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/token":
			authHits++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session_token":  "jwt-from-test",
				"cookie_max_age": 3600,
			})
		case "/sandboxes/":
			listHits++
			if got := r.Header.Get("Authorization"); got != "Bearer jwt-from-test" {
				t.Fatalf("Authorization=%q", got)
			}
			offset := r.URL.Query().Get("offset")
			offsetValue, _ := strconv.Atoi(offset)
			items := []map[string]any{}
			if offset == "0" {
				for i := 0; i < 100; i++ {
					items = append(items, map[string]any{"id": "id", "name": "crabbox-a", "status": "running", "image": "ubuntu"})
				}
			} else if offset == "100" {
				items = append(items, map[string]any{"id": "id", "name": "crabbox-b", "status": "running", "image": "ubuntu"})
			} else {
				t.Fatalf("unexpected offset=%q", offset)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items":  items,
				"total":  101,
				"limit":  100,
				"offset": offsetValue,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	api, err := newIsloClient(Config{Islo: IsloConfig{APIKey: "ak_test", BaseURL: srv.URL}}, Runtime{HTTP: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	items, err := api.ListSandboxes(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 101 {
		t.Fatalf("items=%d", len(items))
	}
	if authHits != 1 || listHits != 2 {
		t.Fatalf("authHits=%d listHits=%d", authHits, listHits)
	}
}
