# events

`crabbox events` prints the coordinator event log for a recorded run.

```sh
crabbox events run_...
crabbox events --id run_...
crabbox events run_... --json
```

Coordinator-backed `crabbox run` creates a durable `run_...` handle before it
leases or syncs. The CLI appends lifecycle events as the run advances through
leasing, bootstrap, sync, command execution, output streaming, finish, and
release.

Human output includes sequence number, event type, phase, stream, timestamp, and
short message or output text. JSON output returns the raw event records.
Output events are a bounded preview: stdout/stderr capture stops after 64 KiB
per run and records an `output.truncated` marker. Use `crabbox logs` for the
retained command output tail.

Related:

- [history](history.md)
- [logs](logs.md)
- [History and logs](../features/history-logs.md)
