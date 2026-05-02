package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
