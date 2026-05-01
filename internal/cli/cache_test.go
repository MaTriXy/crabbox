package cli

import "testing"

func TestParseCacheStats(t *testing.T) {
	entries := parseCacheStats("pnpm\t/var/cache/crabbox/pnpm\t1024\ndocker\t\tImages=1GB,Build Cache=0B\n")
	if len(entries) != 2 {
		t.Fatalf("entries=%#v", entries)
	}
	if entries[0].Kind != "pnpm" || entries[0].Bytes != 1024 {
		t.Fatalf("pnpm entry=%#v", entries[0])
	}
	if entries[1].Kind != "docker" || entries[1].Note == "" {
		t.Fatalf("docker entry=%#v", entries[1])
	}
}
