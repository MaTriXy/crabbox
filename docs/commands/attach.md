# attach

`crabbox attach` follows recorded events for an active coordinator run.

```sh
crabbox attach run_...
crabbox attach --id run_... --after 42
```

Stdout and stderr preview events are written back to stdout and stderr.
Lifecycle events are printed to stderr with their sequence number, phase,
timestamp, and message. When the run has already finished, `attach` prints any
remaining events and exits.

Flags:

```text
--id <run-id>       run id
--after <seq>       resume after this event sequence
--poll <duration>   polling interval, default 1s
```

`attach` follows events emitted by the original CLI. It is not detached command
execution. If the original CLI process dies, the last recorded phase remains
inspectable through [history](history.md), [events](events.md), and
[logs](logs.md).

Output events are a bounded preview. Use [logs](logs.md) for the retained
command output tail after completion.

