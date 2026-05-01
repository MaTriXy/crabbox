package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func (a App) results(ctx context.Context, args []string) error {
	args, jsonAnywhere := extractBoolFlag(args, "json")
	fs := newFlagSet("results", a.Stderr)
	runID := fs.String("id", "", "run id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *runID == "" && fs.NArg() > 0 {
		*runID = fs.Arg(0)
	}
	if *runID == "" {
		return exit(2, "usage: crabbox results <run-id>")
	}
	if jsonAnywhere {
		*jsonOut = true
	}
	coord, err := configuredCoordinator()
	if err != nil {
		return err
	}
	run, err := coord.Run(ctx, *runID)
	if err != nil {
		return err
	}
	if *jsonOut {
		return json.NewEncoder(a.Stdout).Encode(run.Results)
	}
	if run.Results == nil {
		fmt.Fprintf(a.Stdout, "no test results recorded for %s\n", *runID)
		return nil
	}
	printTestResults(a.Stdout, *run.Results)
	return nil
}

func printTestResults(out io.Writer, results TestResultSummary) {
	fmt.Fprintf(out, "results format=%s files=%d suites=%d tests=%d failures=%d errors=%d skipped=%d time=%.3fs\n",
		results.Format, len(results.Files), results.Suites, results.Tests, results.Failures, results.Errors, results.Skipped, results.TimeSeconds)
	if len(results.Failed) == 0 {
		return
	}
	fmt.Fprintln(out, "failed:")
	for _, failure := range results.Failed {
		name := failure.Name
		if failure.Classname != "" {
			name = failure.Classname + "." + name
		}
		location := failure.File
		if location == "" {
			location = failure.Suite
		}
		fmt.Fprintf(out, "  %s %-8s %s", blank(location, "-"), failure.Kind, name)
		msg := strings.TrimSpace(failure.Message)
		if msg != "" {
			fmt.Fprintf(out, " — %s", firstLine(msg))
		}
		fmt.Fprintln(out)
	}
}

func firstLine(value string) string {
	if idx := strings.IndexByte(value, '\n'); idx >= 0 {
		return strings.TrimSpace(value[:idx])
	}
	return value
}
