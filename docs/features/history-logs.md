# History And Logs

Read when:

- changing run recording;
- debugging failed remote commands;
- deciding what belongs in coordinator history.

Coordinator-backed `crabbox run` creates a durable `run_...` handle before
leasing starts. As the CLI advances, it appends ordered events for leasing,
bootstrap, sync, command start, stdout/stderr chunks, command finish, and lease
release. Stdout/stderr events are capped at 64 KiB per run and followed by an
`output.truncated` marker when the cap is reached. When the command exits, the
CLI finishes that run with:

- exit code;
- sync duration;
- command duration;
- total duration;
- owner and org;
- provider, class, and server type;
- retained remote output.

Use:

```sh
crabbox history
crabbox history --lease cbx_...
crabbox events run_...
crabbox attach run_...
crabbox logs run_...
```

In the authenticated browser portal, `/portal/runs/<run-id>` renders the same
run as a human page with command metadata, result summary, recent events, and a
retained log tail. `/portal/runs/<run-id>/logs` stays a plain-text log endpoint,
and `/portal/runs/<run-id>/events` stays JSON for copying or browser-side
inspection.

History records and run events live in the Fleet Durable Object. Log text is
stored separately from run metadata and intentionally capped so noisy commands
cannot exhaust storage. Logs larger than one storage value are chunked by the
coordinator and reassembled by `crabbox logs`. Event output capture is also
bounded; use `crabbox attach` for active run previews and `crabbox logs` for the
retained command output.

Direct-provider mode does not have central history. Use shell output or local terminal logs there.

Related docs:

- [history command](../commands/history.md)
- [attach command](../commands/attach.md)
- [logs command](../commands/logs.md)
- [Observability](../observability.md)
