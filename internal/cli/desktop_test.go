package cli

import (
	"strings"
	"testing"
)

func TestDesktopLaunchRemoteCommandUsesDetachedPOSIXSession(t *testing.T) {
	got := desktopLaunchRemoteCommand(
		SSHTarget{TargetOS: targetLinux},
		"/work/crabbox/cbx_1/repo",
		map[string]string{"DISPLAY": ":99", "BROWSER": "/usr/bin/chromium"},
		[]string{"/usr/bin/chromium", "https://example.com"},
	)
	for _, want := range []string{
		"mkdir -p '/work/crabbox/cbx_1/repo'",
		"cd '/work/crabbox/cbx_1/repo'",
		"DISPLAY=':99'",
		"BROWSER='/usr/bin/chromium'",
		"setsid '/usr/bin/chromium' 'https://example.com'",
		"crabbox-desktop-launch.log",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("desktop launch command missing %q:\n%s", want, got)
		}
	}
}

func TestWindowsDesktopLaunchRemoteCommandUsesInteractiveTask(t *testing.T) {
	got := desktopLaunchRemoteCommand(
		SSHTarget{TargetOS: targetWindows, WindowsMode: windowsModeNormal},
		`C:\crabbox\cbx_1\repo`,
		map[string]string{"BROWSER": `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`},
		[]string{`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`, "https://example.com"},
	)
	for _, want := range []string{
		"CrabboxDesktopLaunch-",
		"windows.username",
		"windows.password",
		"schtasks.exe /Delete",
		`"/IT"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("windows desktop launch command missing %q:\n%s", want, got)
		}
	}
}

func TestWindowsDesktopLaunchScriptStartsAndForegroundsProcess(t *testing.T) {
	got := windowsDesktopLaunchScript(
		`C:\crabbox\cbx_1\repo`,
		map[string]string{"BROWSER": `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`},
		[]string{`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`, "https://example.com"},
	)
	for _, want := range []string{
		`New-Item -ItemType Directory -Force -Path 'C:\crabbox\cbx_1\repo'`,
		`Set-Location -LiteralPath 'C:\crabbox\cbx_1\repo'`,
		`$env:BROWSER = 'C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe'`,
		"Shell.Application",
		"MinimizeAll",
		"Start-Process -FilePath $file",
		"WScript.Shell",
		"AppActivate",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("windows desktop launch script missing %q:\n%s", want, got)
		}
	}
}
