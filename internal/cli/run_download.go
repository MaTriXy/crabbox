package cli

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type runDownloadSpec struct {
	Remote string
	Local  string
}

func parseRunDownloadSpec(value string) (runDownloadSpec, error) {
	remote, local, ok := strings.Cut(strings.TrimSpace(value), "=")
	remote = strings.TrimSpace(remote)
	local = strings.TrimSpace(local)
	if !ok || remote == "" || local == "" {
		return runDownloadSpec{}, exit(2, "--download expects remote=local")
	}
	return runDownloadSpec{Remote: remote, Local: local}, nil
}

func preflightRunLocalOutputs(captureStdout string, downloads []string) error {
	for _, spec := range downloads {
		download, err := parseRunDownloadSpec(spec)
		if err != nil {
			return err
		}
		if err := preflightLocalOutputPath("download "+download.Remote, download.Local, true); err != nil {
			return err
		}
	}
	if captureStdout == "" {
		return nil
	}
	return preflightLocalOutputPath("capture stdout", captureStdout, false)
}

func preflightLocalOutputPath(label, path string, allowMissingDirs bool) error {
	dir := filepath.Dir(path)
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return exit(2, "%s: %s is a directory", label, path)
		}
		return checkWritableFile(label, path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return exit(2, "%s: %v", label, err)
	}
	if dir == "." || dir == "" {
		return checkWritableDir(label, ".")
	}
	if !allowMissingDirs {
		return checkWritableDir(label, dir)
	}
	existing := dir
	for {
		info, err := os.Stat(existing)
		if err == nil {
			if !info.IsDir() {
				return exit(2, "%s: %s is not a directory", label, existing)
			}
			return checkWritableDir(label, existing)
		}
		if !errors.Is(err, os.ErrNotExist) {
			return exit(2, "%s: %v", label, err)
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return exit(2, "%s: %v", label, err)
		}
		existing = parent
	}
}

func checkWritableFile(label, path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return exit(2, "%s: %v", label, err)
	}
	if err := file.Close(); err != nil {
		return exit(2, "%s close: %v", label, err)
	}
	return nil
}

func checkWritableDir(label, dir string) error {
	temp, err := os.CreateTemp(dir, ".crabbox-output-*")
	if err != nil {
		return exit(2, "%s: %v", label, err)
	}
	name := temp.Name()
	closeErr := temp.Close()
	removeErr := os.Remove(name)
	if closeErr != nil {
		return exit(2, "%s close: %v", label, closeErr)
	}
	if removeErr != nil {
		return exit(2, "%s cleanup: %v", label, removeErr)
	}
	return nil
}

func downloadRemoteFile(ctx context.Context, target SSHTarget, workdir, specValue string) (int, string, error) {
	spec, err := parseRunDownloadSpec(specValue)
	if err != nil {
		return 0, "", err
	}
	encoded, err := runSSHOutput(ctx, target, remoteDownloadBase64Command(target, workdir, spec.Remote))
	if err != nil {
		return 0, spec.Local, exit(7, "download %s: %v", spec.Remote, err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(encoded), ""))
	if err != nil {
		return 0, spec.Local, exit(7, "download %s: decode base64: %v", spec.Remote, err)
	}
	if dir := filepath.Dir(spec.Local); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return 0, spec.Local, exit(2, "download %s: create %s: %v", spec.Remote, dir, err)
		}
	}
	if err := os.WriteFile(spec.Local, data, 0o666); err != nil {
		return 0, spec.Local, exit(2, "download %s: write %s: %v", spec.Remote, spec.Local, err)
	}
	return len(data), spec.Local, nil
}

func remoteDownloadBase64Command(target SSHTarget, workdir, remotePath string) string {
	if isWindowsNativeTarget(target) {
		return powershellCommand(`$ErrorActionPreference = "Stop"
Set-Location -LiteralPath ` + psQuote(workdir) + `
$path = ` + psQuote(remotePath) + `
if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "download file not found: $path" }
[Convert]::ToBase64String([System.IO.File]::ReadAllBytes((Resolve-Path -LiteralPath $path).Path))`)
	}
	return fmt.Sprintf("cd %s && test -f %s && base64 < %s", shellQuote(workdir), shellQuote(remotePath), shellQuote(remotePath))
}
