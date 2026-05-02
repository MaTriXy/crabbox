# logs

`crabbox logs` prints the retained remote output tail for a recorded run.

```sh
crabbox logs run_...
crabbox logs --id run_...
crabbox logs run_... --json
```

The plain form writes the log text to stdout. `--json` returns run metadata plus the log.

Logs are bounded tails of remote stdout/stderr. They are for debugging recent runs, not unlimited archival.

Related docs:

- [history](history.md)
- [events](events.md)
- [attach](attach.md)
- [History and logs](../features/history-logs.md)
