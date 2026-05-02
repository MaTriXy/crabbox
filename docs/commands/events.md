# events

`crabbox events` prints the coordinator event log for a recorded run.

```sh
crabbox events run_...
crabbox events --id run_... --after 42 --limit 100
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

Flags:

```text
--id <run-id>       run id
--after <seq>       only show events after this sequence
--limit <n>         default 500, maximum 500
--json              print JSON
```

Related:

- [history](history.md)
- [attach](attach.md)
- [logs](logs.md)
- [History and logs](../features/history-logs.md)
