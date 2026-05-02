package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRunEventStreamWriterCapsOutputEvents(t *testing.T) {
	var events []CoordinatorRunEventInput
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/runs/run_123/events" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var event CoordinatorRunEventInput
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
		_, _ = w.Write([]byte(`{"event":{"runID":"run_123","seq":1,"type":"stdout","createdAt":"2026-05-02T00:00:00Z"}}`))
	}))
	defer server.Close()

	client := &CoordinatorClient{BaseURL: server.URL, Client: server.Client()}
	rec := &runRecorder{coord: client, runID: "run_123", stderr: io.Discard}
	stdout := rec.StreamWriter("stdout")
	chunk := bytes.Repeat([]byte("x"), runEventOutputChunkBytes)
	for i := 0; i < runEventOutputMaxBytes/runEventOutputChunkBytes+10; i++ {
		n, err := stdout.Write(chunk)
		if err != nil {
			t.Fatal(err)
		}
		if n != len(chunk) {
			t.Fatalf("Write returned %d, want %d", n, len(chunk))
		}
	}
	stdout.Flush()
	rec.waitForOutputEvents(time.Second)

	var outputBytes, outputEvents, truncatedEvents int
	for _, event := range events {
		switch event.Type {
		case "stdout":
			outputEvents++
			outputBytes += len(event.Data)
			if len(event.Data) > runEventOutputChunkBytes {
				t.Fatalf("stdout event data length=%d, want <=%d", len(event.Data), runEventOutputChunkBytes)
			}
		case "output.truncated":
			truncatedEvents++
		default:
			t.Fatalf("unexpected event type %q", event.Type)
		}
	}
	if outputBytes != runEventOutputMaxBytes {
		t.Fatalf("outputBytes=%d, want %d", outputBytes, runEventOutputMaxBytes)
	}
	if outputEvents != runEventOutputMaxBytes/runEventOutputChunkBytes {
		t.Fatalf("outputEvents=%d, want %d", outputEvents, runEventOutputMaxBytes/runEventOutputChunkBytes)
	}
	if truncatedEvents != 1 {
		t.Fatalf("truncatedEvents=%d, want 1", truncatedEvents)
	}

	before := len(events)
	if _, err := stdout.Write(chunk); err != nil {
		t.Fatal(err)
	}
	stdout.Flush()
	if len(events) != before {
		t.Fatalf("events after cap=%d, want %d", len(events), before)
	}
}

func TestRunEventStreamWriterDoesNotBlockOnCoordinatorPost(t *testing.T) {
	started := make(chan struct{})
	client := &CoordinatorClient{
		BaseURL: "https://example.test",
		Client:  &http.Client{Transport: blockingRoundTripper{started: started}},
	}
	rec := &runRecorder{coord: client, runID: "run_123", stderr: io.Discard}
	stdout := rec.StreamWriter("stdout")
	chunk := bytes.Repeat([]byte("x"), runEventOutputChunkBytes)

	start := time.Now()
	n, err := stdout.Write(chunk)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(chunk) {
		t.Fatalf("Write returned %d, want %d", n, len(chunk))
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("Write blocked for %s", elapsed)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("output event post did not start")
	}
}

func TestRunRecorderDefersCreateWhenCoordinatorRequiresLeaseID(t *testing.T) {
	var stderr bytes.Buffer
	var createBodies []map[string]any
	var eventBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/runs":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			createBodies = append(createBodies, body)
			if body["leaseID"] == "" {
				http.Error(w, `{"error":"invalid_lease_id"}`, http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"run":{"id":"run_123","leaseID":"cbx_abcdef123456","owner":"peter@example.com","org":"openclaw","provider":"aws","class":"standard","serverType":"t3.small","command":["pnpm","test"],"state":"running","phase":"starting","logBytes":0,"logTruncated":false,"startedAt":"2026-05-02T00:00:00Z"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/runs/run_123/events":
			if err := json.NewDecoder(r.Body).Decode(&eventBody); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(`{"event":{"runID":"run_123","seq":1,"type":"lease.created","createdAt":"2026-05-02T00:00:01Z"}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := &CoordinatorClient{BaseURL: server.URL, Client: server.Client()}
	rec := newRunRecorder(context.Background(), client, Config{
		Provider:   "aws",
		Class:      "standard",
		ServerType: "t3.small",
	}, []string{"pnpm", "test"}, &stderr)
	rec.Event("leasing.started", "leasing", "")
	rec.AttachLease("cbx_abcdef123456", "blue-lobster", Config{
		Provider:   "aws",
		Class:      "standard",
		ServerType: "t3.small",
	})

	if len(createBodies) != 2 {
		t.Fatalf("create requests=%d want 2", len(createBodies))
	}
	if got := createBodies[0]["leaseID"]; got != "" {
		t.Fatalf("first create leaseID=%#v want empty", got)
	}
	if got := createBodies[1]["leaseID"]; got != "cbx_abcdef123456" {
		t.Fatalf("second create leaseID=%#v", got)
	}
	if got := eventBody["type"]; got != "lease.created" {
		t.Fatalf("event body=%#v", eventBody)
	}
	if text := stderr.String(); strings.Contains(text, "warning:") || !strings.Contains(text, "recording run run_123") {
		t.Fatalf("stderr=%q", text)
	}
}

type blockingRoundTripper struct {
	started chan struct{}
}

func (t blockingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	select {
	case <-t.started:
	default:
		close(t.started)
	}
	<-req.Context().Done()
	return nil, context.Cause(req.Context())
}
