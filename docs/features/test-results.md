# Test Results

Read when:

- adding result formats;
- changing how failed tests are summarized;
- debugging why `crabbox results` has no data.

Crabbox can attach JUnit XML summaries to coordinator run history. The agent uses this so a failed run can answer "which tests failed?" without scraping a large log tail.

Configure per run:

```sh
crabbox run --id cbx_... --junit junit.xml -- go test ./...
```

Or per repo:

```yaml
results:
  junit:
    - junit.xml
    - reports/junit.xml
```

After the command exits, the CLI reads those remote files from the workdir, parses JUnit, and sends only the summary to the coordinator. Raw XML is not stored.

Use:

```sh
crabbox history --lease cbx_...
crabbox results run_...
```

Current format support:

- JUnit XML.

Future useful additions:

- Vitest JSON;
- Go `test2json`;
- flaky history across runs;
- changed-file correlation.
