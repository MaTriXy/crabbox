package cli

import (
	"encoding/json"
	"io"
	"time"
)

type timingReport struct {
	Provider      string        `json:"provider"`
	LeaseID       string        `json:"leaseId,omitempty"`
	Slug          string        `json:"slug,omitempty"`
	SyncMs        int64         `json:"syncMs"`
	SyncPhases    []timingPhase `json:"syncPhases,omitempty"`
	SyncSkipped   bool          `json:"syncSkipped"`
	SyncDelegated bool          `json:"syncDelegated,omitempty"`
	CommandMs     int64         `json:"commandMs"`
	TotalMs       int64         `json:"totalMs"`
	ExitCode      int           `json:"exitCode"`
	ActionsRunURL string        `json:"actionsRunUrl,omitempty"`
}

type timingPhase struct {
	Name    string `json:"name"`
	Ms      int64  `json:"ms,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

func writeTimingJSON(w io.Writer, report timingReport) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(report)
}

func timingReportFromRun(provider, leaseID, slug string, timings runTimings, total time.Duration, exitCode int) timingReport {
	return timingReport{
		Provider:    provider,
		LeaseID:     leaseID,
		Slug:        slug,
		SyncMs:      timings.sync.Milliseconds(),
		SyncPhases:  syncTimingPhases(timings.syncSteps),
		SyncSkipped: timings.syncSkipped,
		CommandMs:   timings.command.Milliseconds(),
		TotalMs:     total.Milliseconds(),
		ExitCode:    exitCode,
	}
}

func timingReportFromRunWithActionsURL(provider, leaseID, slug string, timings runTimings, total time.Duration, exitCode int, actionsRunURL string) timingReport {
	report := timingReportFromRun(provider, leaseID, slug, timings, total, exitCode)
	report.ActionsRunURL = actionsRunURL
	return report
}

func syncTimingPhases(steps syncStepTimings) []timingPhase {
	phases := make([]timingPhase, 0, 15)
	appendDuration := func(name string, duration time.Duration) {
		if duration > 0 {
			phases = append(phases, timingPhase{Name: name, Ms: duration.Milliseconds()})
		}
	}
	appendDuration("ssh", steps.sshReady)
	appendDuration("mkdir", steps.mkdir)
	appendDuration("manifest", steps.manifest)
	appendDuration("preflight", steps.preflight)
	appendDuration("fingerprint", steps.fingerprintLocal)
	appendDuration("fingerprint_remote", steps.fingerprintRemote)
	appendDuration("git_seed", steps.gitSeed)
	appendDuration("manifest_write", steps.manifestWrite)
	appendDuration("prune", steps.prune)
	appendDuration("rsync", steps.rsync)
	appendDuration("manifest_apply", steps.manifestApply)
	appendDuration("sanity", steps.sanity)
	appendDuration("git_hydrate", steps.gitHydrate)
	if steps.gitHydrateSkipped {
		phases = append(phases, timingPhase{Name: "git_hydrate", Skipped: true, Reason: steps.gitHydrateSkipReason})
	}
	appendDuration("fingerprint_write", steps.fingerprintWrite)
	return phases
}
