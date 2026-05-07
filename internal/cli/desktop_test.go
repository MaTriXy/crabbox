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
		true,
	)
	for _, want := range []string{
		"mkdir -p '/work/crabbox/cbx_1/repo'",
		"cd '/work/crabbox/cbx_1/repo'",
		"DISPLAY=':99'",
		"BROWSER='/usr/bin/chromium'",
		"setsid '/usr/bin/chromium' 'https://example.com'",
		"crabbox-desktop-launch.log",
		"wmctrl -r :ACTIVE: -b remove,fullscreen",
		"xdotool search --onlyvisible --class google-chrome",
		"windowsize \"$window\" 1500 900",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("desktop launch command missing %q:\n%s", want, got)
		}
	}
}

func TestDesktopTypeUsesPasteForSymbolHeavyText(t *testing.T) {
	for _, text := range []string{"peter@example.com", "token+secret", "line one\nline two", "https://example.com"} {
		if !desktopShouldPasteForType(text) {
			t.Fatalf("expected paste fallback for %q", text)
		}
	}
	if desktopShouldPasteForType("helloWorld123") {
		t.Fatal("plain alphanumeric text should use xdotool type")
	}
}

func TestDesktopPasteRemoteCommandPrefersClipboardTools(t *testing.T) {
	got := desktopPasteRemoteCommand()
	for _, want := range []string{
		"timeout 5s xclip -selection clipboard -loops 1",
		"timeout 5s xsel --clipboard --input",
		"wl-copy --paste-once",
		"xdotool key --clearmodifiers ctrl+v",
		"wait \"$clip_pid\" || true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("paste command missing %q:\n%s", want, got)
		}
	}
}

func TestDesktopKeySequenceArgSkipsLeaseID(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "positional id",
			args: []string{"blue-lobster", "ctrl+l"},
			want: "ctrl+l",
		},
		{
			name: "single dash id",
			args: []string{"-id", "blue-lobster", "ctrl+l"},
			want: "ctrl+l",
		},
		{
			name: "double dash id",
			args: []string{"--id", "blue-lobster", "ctrl+l"},
			want: "ctrl+l",
		},
		{
			name: "equals id",
			args: []string{"--id=blue-lobster", "ctrl+l"},
			want: "ctrl+l",
		},
		{
			name: "explicit keys",
			args: []string{"--id", "blue-lobster", "--keys", "ctrl+l"},
			want: "ctrl+l",
		},
		{
			name: "single dash explicit keys",
			args: []string{"-id", "blue-lobster", "-keys", "ctrl+l"},
			want: "ctrl+l",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := desktopKeySequenceArg(tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("keys=%q, want %q", got, tt.want)
			}
		})
	}
}

func TestDesktopLaunchWebVNCArgsCarriesTargetDetails(t *testing.T) {
	got := desktopLaunchWebVNCArgs(
		Config{Provider: "aws", TargetOS: targetWindows, WindowsMode: windowsModeWSL2, Network: NetworkTailscale},
		SSHTarget{TargetOS: targetWindows, WindowsMode: windowsModeWSL2},
		"cbx_1",
		true,
	)
	joined := strings.Join(got, " ")
	for _, want := range []string{
		"--provider aws",
		"--target windows",
		"--network tailscale",
		"--windows-mode wsl2",
		"--id cbx_1",
		"--open",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("webvnc args missing %q: %v", want, got)
		}
	}
}

func TestDesktopLaunchRemoteCommandCanPassEgressProxyToBrowser(t *testing.T) {
	got := desktopLaunchRemoteCommand(
		SSHTarget{TargetOS: targetLinux},
		"/work/crabbox/cbx_1/repo",
		map[string]string{"DISPLAY": ":99", "BROWSER": "/usr/bin/chromium"},
		[]string{"/usr/bin/chromium", "--proxy-server=http://127.0.0.1:3128", "https://discord.com/login"},
		true,
	)
	if !strings.Contains(got, "'/usr/bin/chromium' '--proxy-server=http://127.0.0.1:3128' 'https://discord.com/login'") {
		t.Fatalf("desktop launch command missing egress proxy arg:\n%s", got)
	}
}

func TestDesktopCommandLooksLikeBrowser(t *testing.T) {
	if !desktopCommandLooksLikeBrowser([]string{"/usr/bin/google-chrome"}, "") {
		t.Fatal("google-chrome should be treated as browser")
	}
	if !desktopCommandLooksLikeBrowser([]string{"/opt/crabbox-browser"}, "/opt/crabbox-browser") {
		t.Fatal("BROWSER env wrapper should be treated as browser")
	}
	if desktopCommandLooksLikeBrowser([]string{"xterm"}, "/opt/crabbox-browser") {
		t.Fatal("xterm should not be treated as browser")
	}
}

func TestDesktopBrowserLaunchCheckAvoidsSelfMatchingShell(t *testing.T) {
	got := desktopBrowserLaunchCheckCommand()
	if strings.Contains(got, "pgrep -f") {
		t.Fatalf("launch check must not match its own shell text:\n%s", got)
	}
	for _, want := range []string{
		"pgrep -x google-chrome",
		"pgrep -x chrome",
		"pgrep -x chromium",
		"pgrep -x chromium-browser",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("launch check missing process-name probe %q:\n%s", want, got)
		}
	}
}

func TestWindowsDesktopLaunchRemoteCommandUsesInteractiveTask(t *testing.T) {
	got := desktopLaunchRemoteCommand(
		SSHTarget{TargetOS: targetWindows, WindowsMode: windowsModeNormal},
		`C:\crabbox\cbx_1\repo`,
		map[string]string{"BROWSER": `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`},
		[]string{`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`, "https://example.com"},
		true,
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
