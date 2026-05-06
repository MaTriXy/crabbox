package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

type runRecorder struct {
	coord           *CoordinatorClient
	command         []string
	runID           string
	stderr          io.Writer
	deferUntilLease bool
	eventsMu        sync.Mutex
	eventsDisabled  bool
	finished        bool
	warned          bool
	warnMu          sync.Mutex
	output          *runOutputEventQueue
	telemetryStart  *LeaseTelemetry
}

func newRunRecorder(ctx context.Context, coord *CoordinatorClient, cfg Config, command []string, stderr io.Writer) *runRecorder {
	rec := &runRecorder{coord: coord, command: command, stderr: stderr}
	if coord == nil {
		return rec
	}
	run, err := coord.CreateRun(ctx, "", cfg, command)
	if err != nil {
		if isInvalidLeaseIDCoordinatorError(err) {
			rec.deferUntilLease = true
			return rec
		}
		rec.warn("run history create failed: %v", err)
		return rec
	}
	rec.attachRun(run)
	return rec
}

func (r *runRecorder) Event(kind, phase, message string) {
	if r == nil || r.runID == "" || (r.finished && kind != "lease.released") {
		return
	}
	r.appendEvent(kind, CoordinatorRunEventInput{
		Type:    kind,
		Phase:   phase,
		Message: message,
	})
}

func (r *runRecorder) appendEvent(kind string, input CoordinatorRunEventInput) {
	if r == nil || r.coord == nil || r.runID == "" || !r.runEventsEnabled() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := r.coord.AppendRunEvent(ctx, r.runID, input)
	if err != nil {
		r.handleRunEventAppendError(kind, err)
	}
}

func (r *runRecorder) AttachLease(leaseID, slug string, cfg Config) {
	if r == nil || r.finished {
		return
	}
	if r.runID == "" && r.deferUntilLease && r.coord != nil && leaseID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		run, err := r.coord.CreateRun(ctx, leaseID, cfg, r.command)
		if err != nil {
			r.warn("run history create failed: %v", err)
			return
		}
		r.attachRun(run)
	}
	if r.runID == "" {
		return
	}
	r.appendEvent("lease.created", CoordinatorRunEventInput{
		Type:        "lease.created",
		Phase:       "leased",
		LeaseID:     leaseID,
		Slug:        slug,
		Provider:    cfg.Provider,
		TargetOS:    cfg.TargetOS,
		WindowsMode: cfg.WindowsMode,
		Class:       cfg.Class,
		ServerType:  cfg.ServerType,
	})
}

func (r *runRecorder) CaptureTelemetryStart(ctx context.Context, target SSHTarget) {
	if r == nil || r.telemetryStart != nil {
		return
	}
	r.telemetryStart = collectLeaseTelemetryBestEffort(ctx, leaseTelemetryCollectorForTarget(target))
}

func (r *runRecorder) attachRun(run CoordinatorRun) {
	r.runID = run.ID
	r.output = newRunOutputEventQueue(r.coord, run.ID, r.handleRunEventAppendError)
	fmt.Fprintf(r.stderr, "recording run %s\n", run.ID)
}

func (r *runRecorder) StreamWriter(stream string) *runEventStreamWriter {
	if r != nil && r.output == nil && r.coord != nil && r.runID != "" {
		r.output = newRunOutputEventQueue(r.coord, r.runID, r.handleRunEventAppendError)
	}
	return &runEventStreamWriter{recorder: r, stream: stream}
}

func (r *runRecorder) Finish(ctx context.Context, target SSHTarget, exitCode int, sync, command time.Duration, log string, truncated bool, results *TestResultSummary) {
	if r == nil || r.runID == "" || r.finished {
		return
	}
	r.waitForOutputEvents(runEventOutputPostWait)
	r.finished = true
	telemetryEnd := collectLeaseTelemetryBestEffort(ctx, leaseTelemetryCollectorForTarget(target))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := r.coord.FinishRun(ctx, r.runID, exitCode, sync, command, log, truncated, results, runTelemetrySummary(r.telemetryStart, telemetryEnd)); err != nil {
		r.warn("run history finish failed for %s: %v", r.runID, err)
	}
}

func (r *runRecorder) Failed(err error) {
	if r == nil || r.runID == "" || r.finished || err == nil {
		return
	}
	r.waitForOutputEvents(runEventOutputPostWait)
	r.finished = true
	r.appendEvent("run.failed", CoordinatorRunEventInput{
		Type:    "run.failed",
		Phase:   "failed",
		Message: err.Error(),
	})
}

func (r *runRecorder) warn(format string, args ...any) {
	if r == nil {
		return
	}
	r.warnMu.Lock()
	defer r.warnMu.Unlock()
	if r.warned {
		return
	}
	r.warned = true
	fmt.Fprintf(r.stderr, "warning: "+format+"\n", args...)
}

func (r *runRecorder) waitForOutputEvents(timeout time.Duration) {
	if r == nil || r.output == nil {
		return
	}
	r.output.CloseAndWait(timeout)
}

func (r *runRecorder) runEventsEnabled() bool {
	r.eventsMu.Lock()
	defer r.eventsMu.Unlock()
	return !r.eventsDisabled
}

func (r *runRecorder) disableRunEvents() {
	r.eventsMu.Lock()
	r.eventsDisabled = true
	r.eventsMu.Unlock()
	if r.output != nil {
		r.output.Disable()
	}
}

func (r *runRecorder) handleRunEventAppendError(kind string, err error) bool {
	if isCoordinatorNotFoundError(err) {
		r.disableRunEvents()
		return false
	}
	r.warn("run event append failed for %s: %v", kind, err)
	return true
}

func isInvalidLeaseIDCoordinatorError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "invalid_lease_id")
}
