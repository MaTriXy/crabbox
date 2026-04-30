package cli

import (
	"strings"
	"testing"
)

func TestRemoteCommandQuotesWorkdirEnvAndArgs(t *testing.T) {
	got := remoteCommand("/work/crabbox/cbx_1/openclaw", map[string]string{"NODE_OPTIONS": "--max-old-space-size=8192"}, []string{"pnpm", "check:changed"})
	for _, want := range []string{
		"cd '/work/crabbox/cbx_1/openclaw'",
		"NODE_OPTIONS='--max-old-space-size=8192'",
		"bash -lc",
		"'exec \"$@\"' bash 'pnpm' 'check:changed'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("remoteCommand() missing %q in %q", want, got)
		}
	}
}

func TestServerTypeForClass(t *testing.T) {
	tests := map[string]string{
		"standard": "ccx33",
		"fast":     "ccx43",
		"large":    "ccx53",
		"beast":    "ccx63",
		"ccx23":    "ccx23",
	}
	for in, want := range tests {
		if got := serverTypeForClass(in); got != want {
			t.Fatalf("serverTypeForClass(%q)=%q want %q", in, got, want)
		}
	}
}
