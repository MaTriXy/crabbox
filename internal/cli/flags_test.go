package cli

import "testing"

func TestExtractBoolFlag(t *testing.T) {
	args, found := extractBoolFlag([]string{"run_123", "--json", "--tail"}, "json")
	if !found {
		t.Fatalf("flag not found")
	}
	if len(args) != 2 || args[0] != "run_123" || args[1] != "--tail" {
		t.Fatalf("args=%v", args)
	}
}

func TestExtractBoolFlagMissing(t *testing.T) {
	args, found := extractBoolFlag([]string{"run_123"}, "json")
	if found {
		t.Fatalf("flag should not be found")
	}
	if len(args) != 1 || args[0] != "run_123" {
		t.Fatalf("args=%v", args)
	}
}
