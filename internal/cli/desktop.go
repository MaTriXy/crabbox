package cli

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

func (a App) desktop(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return exit(2, "usage: crabbox desktop launch --id <lease-id-or-slug> [--browser] [--url <url>] -- <command...>")
	}
	switch args[0] {
	case "launch":
		return a.desktopLaunch(ctx, args[1:])
	default:
		return exit(2, "unknown desktop command %q", args[0])
	}
}

func (a App) desktopLaunch(ctx context.Context, args []string) error {
	defaults := defaultConfig()
	fs := newFlagSet("desktop launch", a.Stderr)
	provider := fs.String("provider", defaults.Provider, "provider: hetzner, aws, or ssh")
	id := fs.String("id", "", "lease id or slug")
	browser := fs.Bool("browser", false, "launch the target browser")
	url := fs.String("url", "", "URL to pass to the launched browser")
	reclaim := fs.Bool("reclaim", false, "claim this lease for the current repo")
	targetFlags := registerTargetFlags(fs, defaults)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	positionalID := false
	if *id == "" && fs.NArg() > 0 {
		*id = fs.Arg(0)
		positionalID = true
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.Provider = *provider
	cfg.Desktop = true
	cfg.Browser = *browser
	if err := applyTargetFlagOverrides(&cfg, fs, targetFlags); err != nil {
		return err
	}
	if err := validateRequestedCapabilities(cfg); err != nil {
		return err
	}
	if *id == "" && !isStaticProvider(cfg.Provider) {
		return exit(2, "usage: crabbox desktop launch --id <lease-id-or-slug> [--browser] [--url <url>] -- <command...>")
	}
	server, target, leaseID, err := a.resolveLeaseTarget(ctx, cfg, *id)
	if err != nil {
		return err
	}
	if err := enforceManagedLeaseCapabilities(cfg, server, leaseID); err != nil {
		return err
	}
	repo, err := findRepo()
	if err != nil {
		return err
	}
	if err := claimLeaseForRepoConfig(leaseID, serverSlug(server), cfg, repo.Root, cfg.IdleTimeout, *reclaim); err != nil {
		return err
	}
	a.touchActiveLeaseBestEffort(ctx, cfg, server, leaseID)
	if err := waitForLoopbackVNC(ctx, &target); err != nil {
		return err
	}
	env, err := requestedCapabilityEnv(ctx, cfg, target)
	if err != nil {
		return err
	}
	command := fs.Args()
	if positionalID && len(command) > 0 && command[0] == *id {
		command = command[1:]
	}
	if *browser {
		if len(command) == 0 {
			if env["BROWSER"] == "" {
				return exit(2, "browser=true requested but target did not report BROWSER")
			}
			command = []string{env["BROWSER"]}
			if strings.TrimSpace(*url) != "" {
				command = append(command, strings.TrimSpace(*url))
			}
		} else if strings.TrimSpace(*url) != "" {
			command = append(command, strings.TrimSpace(*url))
		}
	}
	if len(command) == 0 {
		return exit(2, "usage: crabbox desktop launch --id <lease-id-or-slug> -- <command...>")
	}
	workdir := remoteJoin(cfg, leaseID, repo.Name)
	if err := runSSHQuiet(ctx, target, desktopLaunchRemoteCommand(target, workdir, env, command)); err != nil {
		return exit(5, "launch desktop command: %v", err)
	}
	fmt.Fprintf(a.Stdout, "launched: %s\n", strings.Join(command, " "))
	return nil
}

func desktopLaunchRemoteCommand(target SSHTarget, workdir string, env map[string]string, command []string) string {
	if isWindowsNativeTarget(target) {
		return windowsDesktopLaunchRemoteCommand(workdir, env, command)
	}
	if target.TargetOS == targetMacOS {
		return posixDesktopLaunchRemoteCommand(workdir, env, command)
	}
	return posixDesktopLaunchRemoteCommand(workdir, env, command)
}

func posixDesktopLaunchRemoteCommand(workdir string, env map[string]string, command []string) string {
	var b bytes.Buffer
	b.WriteString("set -eu\n")
	if workdir != "" {
		b.WriteString("cd " + shellQuote(workdir) + "\n")
	}
	for key, value := range env {
		b.WriteString(key + "=" + shellQuote(value) + "\n")
		b.WriteString("export " + key + "\n")
	}
	b.WriteString("log=${TMPDIR:-/tmp}/crabbox-desktop-launch.log\n")
	b.WriteString("if command -v setsid >/dev/null 2>&1; then\n")
	b.WriteString("  setsid ")
	writeShellArgv(&b, command)
	b.WriteString(" >\"$log\" 2>&1 < /dev/null &\n")
	b.WriteString("else\n")
	b.WriteString("  nohup ")
	writeShellArgv(&b, command)
	b.WriteString(" >\"$log\" 2>&1 < /dev/null &\n")
	b.WriteString("fi\n")
	return b.String()
}

func writeShellArgv(b *bytes.Buffer, command []string) {
	for i, arg := range command {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(shellQuote(arg))
	}
}

func windowsDesktopLaunchRemoteCommand(workdir string, env map[string]string, command []string) string {
	inner := windowsDesktopLaunchScript(workdir, env, command)
	return `$ErrorActionPreference = "Stop"
$base = "C:\ProgramData\crabbox"
$usernamePath = Join-Path $base "windows.username"
$passwordPath = Join-Path $base "windows.password"
$username = if (Test-Path -LiteralPath $usernamePath) { Get-Content -Raw -LiteralPath $usernamePath } else { $env:USERNAME }
$username = $username.Trim()
$password = if (Test-Path -LiteralPath $passwordPath) { (Get-Content -Raw -LiteralPath $passwordPath).Trim() } else { "" }
$taskName = "CrabboxDesktopLaunch-" + [Guid]::NewGuid().ToString("N")
$script = Join-Path $base ($taskName + ".ps1")
Set-Content -Encoding UTF8 -LiteralPath $script -Value ` + psQuote(inner) + `
cmd.exe /c "schtasks.exe /Delete /TN $taskName /F 2>NUL" | Out-Null
$startTime = (Get-Date).AddMinutes(1).ToString("HH:mm")
$createArgs = @("/Create", "/TN", $taskName, "/SC", "ONCE", "/ST", $startTime, "/TR", "powershell.exe -NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File $script", "/RU", $username, "/IT", "/F")
& schtasks.exe @createArgs | Out-Null
if ($LASTEXITCODE -ne 0 -and $password -ne "") {
  & schtasks.exe @($createArgs + @("/RP", $password)) | Out-Null
}
if ($LASTEXITCODE -ne 0) { throw "failed to create interactive desktop launch task" }
& schtasks.exe /Run /TN $taskName | Out-Null
Start-Sleep -Seconds 2
& schtasks.exe /Delete /TN $taskName /F | Out-Null
Remove-Item -Force -LiteralPath $script -ErrorAction SilentlyContinue
`
}

func windowsDesktopLaunchScript(workdir string, env map[string]string, command []string) string {
	var b bytes.Buffer
	b.WriteString("$ErrorActionPreference = \"Stop\"\n")
	if workdir != "" {
		b.WriteString("Set-Location -LiteralPath " + psQuote(workdir) + "\n")
	}
	for key, value := range env {
		b.WriteString("$env:" + key + " = " + psQuote(value) + "\n")
	}
	b.WriteString("$file = " + psQuote(command[0]) + "\n")
	b.WriteString("$arguments = @(")
	for i, arg := range command[1:] {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(psQuote(arg))
	}
	b.WriteString(")\n")
	b.WriteString(`try {
  $shell = New-Object -ComObject Shell.Application
  $shell.MinimizeAll()
  Start-Sleep -Milliseconds 250
} catch {}
$process = Start-Process -FilePath $file -ArgumentList $arguments -WorkingDirectory (Get-Location).Path -WindowStyle Normal -PassThru
Start-Sleep -Seconds 2
try {
  $wshell = New-Object -ComObject WScript.Shell
  $names = @()
  if ($process -and $process.ProcessName) { $names += $process.ProcessName }
  $names += [IO.Path]::GetFileNameWithoutExtension($file)
  foreach ($name in ($names | Where-Object { $_ } | Select-Object -Unique)) {
    if ($wshell.AppActivate($name)) { break }
  }
} catch {}
`)
	return b.String()
}
