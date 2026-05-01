# results

`crabbox results` prints structured test summaries attached to a recorded run.

```sh
crabbox run --id cbx_... --junit junit.xml -- go test ./...
crabbox results run_...
crabbox results run_... --json
```

Results are attached only when `crabbox run` is told where to find remote JUnit XML. Use either:

```sh
crabbox run --junit junit.xml -- <command...>
```

or repo config:

```yaml
results:
  junit:
    - junit.xml
    - reports/junit.xml
```

Human output shows totals and failed test cases. JSON output returns the stored summary.

Related docs:

- [run](run.md)
- [history](history.md)
- [Test results](../features/test-results.md)
