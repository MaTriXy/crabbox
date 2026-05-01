package cli

import (
	"context"
	"fmt"
	"strings"
)

const resultFileMarker = "__CRABBOX_RESULT_FILE__:"

func collectRemoteJUnitResults(ctx context.Context, target SSHTarget, workdir string, paths []string) (*TestResultSummary, error) {
	paths = appendUniqueStrings(nil, paths...)
	if len(paths) == 0 {
		return nil, nil
	}
	out, err := runSSHOutput(ctx, target, remoteReadResultFiles(workdir, paths))
	if err != nil {
		return nil, err
	}
	files := parseMarkedFiles(out)
	return parseJUnitResults(files)
}

func remoteReadResultFiles(workdir string, paths []string) string {
	var b strings.Builder
	b.WriteString("cd ")
	b.WriteString(shellQuote(workdir))
	b.WriteString(" && ")
	b.WriteString("for f in")
	for _, path := range paths {
		b.WriteByte(' ')
		b.WriteString(shellQuote(path))
	}
	b.WriteString("; do if [ -f \"$f\" ]; then printf '\\n")
	b.WriteString(resultFileMarker)
	b.WriteString("%s\\n' \"$f\"; cat \"$f\"; fi; done")
	return b.String()
}

func parseMarkedFiles(output string) map[string]string {
	files := map[string]string{}
	current := ""
	var b strings.Builder
	flush := func() {
		if current != "" {
			files[current] = strings.TrimSpace(b.String())
			b.Reset()
		}
	}
	for _, line := range strings.Split(output, "\n") {
		if name, ok := strings.CutPrefix(line, resultFileMarker); ok {
			flush()
			current = strings.TrimSpace(name)
			continue
		}
		if current != "" {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	flush()
	return files
}

func resultSummaryLine(results *TestResultSummary) string {
	if results == nil {
		return ""
	}
	return fmt.Sprintf("test results files=%d tests=%d failures=%d errors=%d skipped=%d", len(results.Files), results.Tests, results.Failures, results.Errors, results.Skipped)
}
