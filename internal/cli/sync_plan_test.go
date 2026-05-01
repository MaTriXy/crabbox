package cli

import "testing"

func TestSyncPlanDir(t *testing.T) {
	tests := map[string]string{
		"README.md":             ".",
		"docs/README.md":        "docs",
		"packages/app/src/a.ts": "packages/app",
		"apps/foo/.build/a.o":   "apps/foo",
		"worker/src/index.ts":   "worker/src",
	}
	for input, want := range tests {
		if got := syncPlanDir(input); got != want {
			t.Fatalf("syncPlanDir(%q)=%q want %q", input, got, want)
		}
	}
}

func TestSortSyncPlanRows(t *testing.T) {
	rows := []syncPlanRow{{Path: "b", Bytes: 2}, {Path: "a", Bytes: 2}, {Path: "c", Bytes: 3}}
	sortSyncPlanRows(rows)
	got := rows[0].Path + rows[1].Path + rows[2].Path
	if got != "cab" {
		t.Fatalf("sorted rows=%v", rows)
	}
}
