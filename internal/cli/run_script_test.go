package cli

import (
	"strings"
	"testing"
)

func TestRemoteRunScriptCommandUsesUploadedFile(t *testing.T) {
	spec := &RunScriptSpec{
		Source:     "live.sh",
		RemotePath: ".crabbox/scripts/abc-live.sh",
		Shebang:    true,
	}
	got := remoteRunScriptCommandWithEnvFile("/work/repo", map[string]string{"OPENAI_API_KEY": "sk-test"}, "", spec, []string{"arg one"})
	for _, want := range []string{
		"cd '/work/repo'",
		"OPENAI_API_KEY='sk-test'",
		"exec \"$@\"",
		"'.crabbox/scripts/abc-live.sh'",
		"'arg one'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("remote command missing %q in %q", want, got)
		}
	}
}

func TestRemoteRunScriptCommandWithoutShebangUsesBash(t *testing.T) {
	spec := &RunScriptSpec{RemotePath: ".crabbox/scripts/abc-script.sh"}
	got := remoteRunScriptCommandWithEnvFile("/work/repo", nil, "", spec, nil)
	if !strings.Contains(got, `exec bash "$@"`) {
		t.Fatalf("remote command should run script through bash: %q", got)
	}
}

func TestRunScriptRecordCommand(t *testing.T) {
	got := runScriptRecordCommand(&RunScriptSpec{Source: "./smoke.sh"}, []string{"--flag"})
	if strings.Join(got, " ") != "--script ./smoke.sh --flag" {
		t.Fatalf("record command=%q", got)
	}
	got = runScriptRecordCommand(&RunScriptSpec{Source: "stdin"}, nil)
	if strings.Join(got, " ") != "--script-stdin" {
		t.Fatalf("stdin record command=%q", got)
	}
}

func TestSafeScriptNameKeepsBasenameAndHash(t *testing.T) {
	got := safeScriptName("../bad live.sh", "abc123")
	if got != "abc123-badlive.sh" {
		t.Fatalf("safe name=%q", got)
	}
}
