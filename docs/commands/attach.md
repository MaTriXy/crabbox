# attach

`crabbox attach` follows recorded events for an active coordinator run.

```sh
crabbox attach run_abcdef123456
crabbox attach --id run_abcdef123456 --after 42
crabbox attach run_abcdef123456 --poll 500ms
```

## Behavior

`attach` follows coordinator run events, prints them as they arrive, and exits
when the run finishes. Newer brokers stream events over the authenticated
coordinator control WebSocket; older brokers or dropped sockets fall back to
the HTTPS events API from the last printed sequence.

- stdout and stderr preview events are written back to stdout and stderr,
  preserving the stream split;
- lifecycle events (lease, bootstrap, sync, command-start, finish, release)
  are printed to stderr with their sequence number, phase, timestamp, and
  message;
- when the run has already finished, attach prints any remaining events
  and exits;
- when a WebSocket attach starts behind the run's current event sequence,
  attach drains the backlog in bounded pages before waiting for live pushes;
- when the run is still active, attach waits for streamed events or polls until
  it sees the run finish.

`attach` is not detached command execution. It follows the events the
original CLI is emitting; if that CLI process dies, the run state remains
inspectable through [history](history.md), [events](events.md), and
[logs](logs.md), but `attach` cannot resurrect it.

## Bounded Output

Output events are a bounded preview. The coordinator caps stdout/stderr
capture at 64 KiB per run and records an `output.truncated` marker when the
cap is reached. Use [logs](logs.md) for the larger retained command output
after completion.

## Flags

```text
--id <run-id>       run id (also accepted as a positional argument)
--after <seq>       resume after this event sequence number
--poll <duration>   fallback poll interval and WebSocket idle check, default 1s
```

## Use Cases

- watch a long warmup or run from a second terminal without disturbing the
  original CLI;
- monitor an agent-launched run while doing something else locally;
- replay events from a known sequence (`--after`) when reconnecting after
  a network blip.

## Direct Mode

Direct-provider mode does not record runs centrally, so `attach` has no
event stream to follow. Use shell output from the original CLI instead.

Related docs:

- [logs](logs.md)
- [events](events.md)
- [history](history.md)
- [run](run.md)
- [History and logs](../features/history-logs.md)
