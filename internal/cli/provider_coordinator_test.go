package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCoordinatorListFallsBackToUserLeasesWhenAdminTokenUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/pool":
			if got := r.Header.Get("Authorization"); got != "Bearer stale-admin-token" {
				t.Fatalf("pool auth=%q", got)
			}
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		case "/v1/leases":
			if got := r.URL.Query().Get("state"); got != "active" {
				t.Fatalf("leases state=%q", got)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer user-token" {
				t.Fatalf("leases auth=%q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"leases": []CoordinatorLease{
				{
					ID:                 "cbx_123",
					Slug:               "blue-lobster",
					Provider:           "aws",
					TargetOS:           targetLinux,
					ServerID:           42,
					CloudID:            "i-123",
					ServerName:         "crabbox-blue-lobster",
					Host:               "203.0.113.10",
					SSHUser:            "crabbox",
					SSHPort:            "2222",
					ServerType:         "c7a.48xlarge",
					State:              "active",
					Keep:               true,
					ExpiresAt:          "2026-05-07T15:00:00Z",
					IdleTimeoutSeconds: 1800,
				},
				{ID: "cbx_other", Provider: "hetzner", State: "active"},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var stderr bytes.Buffer
	cfg := Config{
		Provider:        "aws",
		TargetOS:        targetLinux,
		Coordinator:     server.URL,
		CoordToken:      "user-token",
		CoordAdminToken: "stale-admin-token",
	}
	coord, _, err := newCoordinatorClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	backend := &coordinatorLeaseBackend{cfg: cfg, coord: coord, rt: Runtime{Stderr: &stderr}}

	servers, err := backend.List(context.Background(), ListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("servers=%d, want 1: %#v", len(servers), servers)
	}
	if servers[0].Labels["lease"] != "cbx_123" || servers[0].Labels["slug"] != "blue-lobster" {
		t.Fatalf("server labels=%#v", servers[0].Labels)
	}
	if !strings.Contains(stderr.String(), "falling back to user-visible leases") {
		t.Fatalf("missing fallback warning: %q", stderr.String())
	}
}

func TestCoordinatorListJSONFallsBackWhenAdminTokenMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/leases" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("state"); got != "active" {
			t.Fatalf("leases state=%q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"leases": []CoordinatorLease{
			{ID: "cbx_123", Provider: "aws", State: "active"},
		}})
	}))
	defer server.Close()

	cfg := Config{Provider: "aws", TargetOS: targetLinux, Coordinator: server.URL, CoordToken: "user-token"}
	coord, _, err := newCoordinatorClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	backend := &coordinatorLeaseBackend{cfg: cfg, coord: coord, rt: Runtime{Stderr: &bytes.Buffer{}}}

	view, err := backend.ListJSON(context.Background(), ListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	leases, ok := view.([]CoordinatorLease)
	if !ok {
		t.Fatalf("view=%T, want []CoordinatorLease", view)
	}
	if len(leases) != 1 || leases[0].ID != "cbx_123" {
		t.Fatalf("leases=%#v", leases)
	}
}
