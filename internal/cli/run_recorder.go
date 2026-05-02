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
	finished        bool
	warned          bool
	warnMu          sync.Mutex
	output          *runOutputEventQueue
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := r.coord.AppendRunEvent(ctx, r.runID, CoordinatorRunEventInput{
		Type:    kind,
		Phase:   phase,
		Message: message,
	})
	if err != nil {
		r.warn("run event append failed for %s: %v", kind, err)
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := r.coord.AppendRunEvent(ctx, r.runID, CoordinatorRunEventInput{
		Type:       "lease.created",
		Phase:      "leased",
		LeaseID:    leaseID,
		Slug:       slug,
		Provider:   cfg.Provider,
		Class:      cfg.Class,
		ServerType: cfg.ServerType,
	})
	if err != nil {
		r.warn("run event append failed for lease.created: %v", err)
	}
}

func (r *runRecorder) attachRun(run CoordinatorRun) {
	r.runID = run.ID
	r.output = newRunOutputEventQueue(r.coord, run.ID, r.warn)
	fmt.Fprintf(r.stderr, "recording run %s\n", run.ID)
}

func (r *runRecorder) StreamWriter(stream string) *runEventStreamWriter {
	if r != nil && r.output == nil && r.coord != nil && r.runID != "" {
		r.output = newRunOutputEventQueue(r.coord, r.runID, r.warn)
	}
	return &runEventStreamWriter{recorder: r, stream: stream}
}

func (r *runRecorder) Finish(exitCode int, sync, command time.Duration, log string, truncated bool, results *TestResultSummary) {
	if r == nil || r.runID == "" || r.finished {
		return
	}
	r.waitForOutputEvents(runEventOutputPostWait)
	r.finished = true
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := r.coord.FinishRun(ctx, r.runID, exitCode, sync, command, log, truncated, results); err != nil {
		r.warn("run history finish failed for %s: %v", r.runID, err)
	}
}

func (r *runRecorder) Failed(err error) {
	if r == nil || r.runID == "" || r.finished || err == nil {
		return
	}
	r.waitForOutputEvents(runEventOutputPostWait)
	r.finished = true
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, appendErr := r.coord.AppendRunEvent(ctx, r.runID, CoordinatorRunEventInput{
		Type:    "run.failed",
		Phase:   "failed",
		Message: err.Error(),
	})
	if appendErr != nil {
		r.warn("run event append failed for run.failed: %v", appendErr)
	}
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

func isInvalidLeaseIDCoordinatorError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "invalid_lease_id")
}
