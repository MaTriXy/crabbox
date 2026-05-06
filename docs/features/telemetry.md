# Telemetry

Read when:

- changing how Crabbox samples runner load, memory, disk, or uptime;
- adding new metrics to lease records or run history;
- debugging missing portal sparklines or stale telemetry pills;
- understanding where telemetry stops and full observability begins.

Crabbox captures lightweight runner telemetry so a lease detail page or run
record can answer "is this box healthy right now?" and "did this command spike
memory?" without standing up Prometheus or shipping a logging agent. Telemetry
is best-effort, capped, and only exists for managed Linux leases.

## What Gets Captured

For Linux runners, the CLI runs a small remote script through the lease SSH
target whenever it has a reason to talk to the box (heartbeat,
warmup-complete, status check, mid-run sample). The script reads:

- `load1`, `load5`, `load15` from `/proc/loadavg`;
- `memoryTotalBytes`, `memoryUsedBytes`, `memoryPercent` derived from
  `MemTotal` and `MemAvailable` in `/proc/meminfo`;
- `diskTotalBytes`, `diskUsedBytes`, `diskPercent` from `df -PB1 /`;
- `uptimeSeconds` from `/proc/uptime`.

Each sample is parsed into a `LeaseTelemetry` record:

```json
{
  "capturedAt": "2026-05-07T07:42:18Z",
  "source": "ssh-linux",
  "load1": 0.42,
  "load5": 0.30,
  "load15": 0.18,
  "memoryUsedBytes": 5368709120,
  "memoryTotalBytes": 16777216000,
  "memoryPercent": 32.0,
  "diskUsedBytes": 21474836480,
  "diskTotalBytes": 107374182400,
  "diskPercent": 20.0,
  "uptimeSeconds": 38400
}
```

Non-Linux targets (managed Windows, EC2 Mac, static SSH macOS/Windows) are
intentionally excluded from telemetry capture today. The collector returns
`nil` for non-Linux targets and the coordinator silently skips storing
samples for them.

## Where It Lives

Telemetry lives in two places on the coordinator:

- **Lease record.** The Fleet Durable Object stores the most recent sanitized
  snapshot on the lease (`telemetry`) and a bounded ring of the latest 60
  samples (`telemetryHistory`). The ring is keyed by `capturedAt`; older
  samples drop off as new ones arrive.
- **Run record.** When a `run_...` is in progress, the CLI POSTs samples to
  `/v1/runs/{run-id}/telemetry`. The run record keeps a bounded `start`,
  `end`, and a small `samples[]` array so longer commands have a short
  load/memory/disk trend instead of just two endpoints.

Static SSH and delegated providers do not produce telemetry. Their lease
records have no `telemetry` field; their portal rows render a quiet "no
telemetry" pill.

## How Samples Get Sent

The CLI samples in three contexts:

1. **Heartbeat.** While a command runs, the heartbeat goroutine asks for a
   fresh sample with a short 5-second timeout, attaches it to the heartbeat
   body, and lets the coordinator update the lease record and append to the
   ring. Heartbeats that fail to collect just send no telemetry; the command
   keeps running.
2. **Warmup and status.** `crabbox warmup`, `crabbox status`, and
   `crabbox inspect` collect a one-off sample so the user sees current load
   on the same line that prints lease state.
3. **Run telemetry.** Long commands periodically post samples through the run
   telemetry endpoint while the command is active; the run record captures
   start, end, and a trimmed series.

All collection runs through `collectLeaseTelemetryBestEffort`, which wraps the
collector in a 5-second timeout. A failed sample is never an error - it's a
signal that the box was busy or temporarily unreachable.

## What Shows Up Where

- **`crabbox status --id ...`**: prints `load=0.42 mem=5.0GiB/16.0GiB
  disk=20.0GiB/100.0GiB uptime=10h40m telemetry=2s` when a sample is
  available. Older samples render as `telemetry=4m12s` so freshness is
  obvious at a glance.
- **`crabbox history`** and **`crabbox events`**: include start/end snapshots
  plus a memory delta on completed runs.
- **`/portal/leases/{id-or-slug}`**: shows the latest sample as gauges and
  renders load, memory, and disk sparklines when more than one sample is
  present. Stale samples (>5 minutes) get a `stale telemetry` pill;
  high-resource samples get `high load`, `high memory`, or `high disk` pills
  on the same row.
- **`/portal/runs/{run-id}`**: renders a compact resource delta line and short
  trend lines for runs with mid-run samples.

The coordinator never serves raw `/proc` content - only the parsed numeric
fields above. Tests assert that hostnames, kernel versions, mount points, and
process tables never reach storage.

## Limits And Defaults

- Sampler timeout: 5 seconds per call.
- Lease telemetry ring: 60 samples per lease.
- Run telemetry samples: bounded to a small ring (start, end, plus a small
  middle series) and serialized once on `POST /v1/runs/{run-id}/finish`.
- High-resource pill thresholds: load > number of CPUs, memory percent > 90,
  disk percent > 90.
- Stale telemetry threshold: 5 minutes since `capturedAt`.

These thresholds are operational hints, not alerts - Crabbox does not page or
auto-action on telemetry. Use observability tooling for that.

## When To Use Full Observability Instead

Telemetry is intentionally narrow. It is a "is the box healthy?" pulse, not a
metrics pipeline. For per-process traces, per-command flame graphs, or
historical correlations across many runs, scrape the runner with a real
agent or ship logs to a real backend. Crabbox does not try to replace that
layer; see [Observability](../observability.md) for what we plumb upstream.

## Configuration

Telemetry has no user-facing toggle. Disabling it would not save meaningful
runtime but would remove the most useful health signal in the portal. There
is no env flag to silence sampling.

If you need to extend the captured fields, add them in:

- the parser in `internal/cli/telemetry.go`;
- the coordinator schema in `worker/src/types.ts`;
- the lease/run portal renderers in `worker/src/portal.ts`;
- the storage in `worker/src/fleet.ts`.

Keep new fields numeric, sanitized, and bounded. Free-form strings, hostnames,
and process names do not belong on the telemetry record.

Related docs:

- [Coordinator](coordinator.md)
- [Orchestrator](../orchestrator.md)
- [History and logs](history-logs.md)
- [Observability](../observability.md)
- [Source map](../source-map.md)
