package cli

import (
	"strings"
	"testing"
)

func TestDefaultScreenshotPath(t *testing.T) {
	if got := defaultScreenshotPath("cbx_123", "Blue Lobster"); got != "crabbox-blue-lobster-screenshot.png" {
		t.Fatalf("path=%q", got)
	}
	if got := defaultScreenshotPath("cbx_123", ""); got != "crabbox-cbx-123-screenshot.png" {
		t.Fatalf("fallback path=%q", got)
	}
}

func TestScreenshotRemoteCommandUsesDesktopDisplayAndPNG(t *testing.T) {
	got := screenshotRemoteCommand()
	for _, want := range []string{
		`DISPLAY="${DISPLAY:-:99}"`,
		"command -v scrot",
		"scrot -z -o",
		"cat \"$tmp\"",
		"import -window root png:-",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("screenshot command missing %q:\n%s", want, got)
		}
	}
}
