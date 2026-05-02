package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEventsCommandPassesPagination(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.String()
		if r.Method != http.MethodGet || r.URL.Path != "/v1/runs/run_123/events" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"events":[{"runID":"run_123","seq":5,"type":"sync.finished","phase":"synced","createdAt":"2026-05-02T00:00:00Z"}]}`))
	}))
	defer server.Close()
	t.Setenv("CRABBOX_COORDINATOR", server.URL)
	t.Setenv("CRABBOX_COORDINATOR_TOKEN", "")

	var stdout, stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr}
	if err := app.events(context.Background(), []string{"run_123", "--after", "4", "--limit", "25"}); err != nil {
		t.Fatal(err)
	}
	if path != "/v1/runs/run_123/events?after=4&limit=25" {
		t.Fatalf("path=%q", path)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("sync.finished")) {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestAttachCommandReplaysOutputAndStopsWhenRunFinished(t *testing.T) {
	eventCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/runs/run_123/events":
			eventCalls++
			if eventCalls == 1 {
				if got := r.URL.Query().Get("after"); got != "" {
					t.Fatalf("first after=%q", got)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"events": []map[string]any{
					{"runID": "run_123", "seq": 1, "type": "stdout", "stream": "stdout", "data": "hello\n", "createdAt": "2026-05-02T00:00:00Z"},
					{"runID": "run_123", "seq": 2, "type": "stderr", "stream": "stderr", "data": "warn\n", "createdAt": "2026-05-02T00:00:01Z"},
				}})
				return
			}
			if got := r.URL.Query().Get("after"); got != "2" {
				t.Fatalf("next after=%q", got)
			}
			_, _ = w.Write([]byte(`{"events":[]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/runs/run_123":
			_, _ = w.Write([]byte(`{"run":{"id":"run_123","leaseID":"cbx_123","owner":"peter@example.com","org":"openclaw","provider":"aws","class":"standard","serverType":"t3.small","command":["true"],"state":"succeeded","phase":"finished","logBytes":0,"logTruncated":false,"startedAt":"2026-05-02T00:00:00Z"}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	t.Setenv("CRABBOX_COORDINATOR", server.URL)
	t.Setenv("CRABBOX_COORDINATOR_TOKEN", "")

	var stdout, stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr}
	if err := app.attach(context.Background(), []string{"run_123", "--poll", "1ms"}); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "hello\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if stderr.String() != "warn\n" {
		t.Fatalf("stderr=%q", stderr.String())
	}
	if eventCalls != 2 {
		t.Fatalf("eventCalls=%d, want 2", eventCalls)
	}
}
