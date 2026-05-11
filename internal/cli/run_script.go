package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type RunScriptSpec struct {
	Source     string
	Data       []byte
	RemotePath string
	Shebang    bool
}

func loadRunScript(path string, stdin bool, input io.Reader) (*RunScriptSpec, error) {
	if path != "" && stdin {
		return nil, exit(2, "--script and --script-stdin are mutually exclusive")
	}
	if path == "" && !stdin {
		return nil, nil
	}
	source := path
	var data []byte
	var err error
	if stdin {
		source = "stdin"
		if input == nil {
			input = os.Stdin
		}
		data, err = io.ReadAll(input)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, exit(2, "read script %s: %v", source, err)
	}
	if len(data) == 0 {
		return nil, exit(2, "script %s is empty", source)
	}
	sum := sha256.Sum256(data)
	name := safeScriptName(source, hex.EncodeToString(sum[:])[:12])
	return &RunScriptSpec{
		Source:     source,
		Data:       data,
		RemotePath: ".crabbox/scripts/" + name,
		Shebang:    strings.HasPrefix(string(data), "#!"),
	}, nil
}

func safeScriptName(source, prefix string) string {
	base := "script.sh"
	if source != "" && source != "stdin" {
		base = filepath.Base(source)
	}
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '_' || r == '-':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		b.WriteString("script.sh")
	}
	return prefix + "-" + b.String()
}

func uploadRunScript(ctx context.Context, target SSHTarget, workdir string, spec *RunScriptSpec) error {
	if spec == nil {
		return nil
	}
	remote := remoteUploadRunScriptCommand(workdir, spec.RemotePath)
	if err := runSSHInput(ctx, target, remote, bytes.NewReader(spec.Data), io.Discard, io.Discard); err != nil {
		return exit(7, "upload script: %v", err)
	}
	return nil
}

func remoteUploadRunScriptCommand(workdir, remotePath string) string {
	dir := filepath.ToSlash(filepath.Dir(remotePath))
	script := "set -eu\n" +
		"cd " + shellQuote(workdir) + "\n" +
		"mkdir -p " + shellQuote(dir) + "\n" +
		"cat > " + shellQuote(remotePath) + "\n" +
		"chmod 700 " + shellQuote(remotePath) + "\n"
	return "bash -lc " + shellQuote(script)
}

func remoteRunScriptCommandWithEnvFile(workdir string, env map[string]string, envFile string, script *RunScriptSpec, args []string) string {
	return remoteRunScriptCommandWithEnvFiles(workdir, env, singleEnvFile(envFile), script, args)
}

func remoteRunScriptCommandWithEnvFiles(workdir string, env map[string]string, envFiles []string, script *RunScriptSpec, args []string) string {
	var b strings.Builder
	writeRemoteCommandPrefix(&b, workdir, env, envFiles)
	if script.Shebang {
		b.WriteString("bash -lc ")
		b.WriteString(shellQuote(`exec "$@"`))
		b.WriteString(" bash ")
	} else {
		b.WriteString("bash -lc ")
		b.WriteString(shellQuote(`exec bash "$@"`))
		b.WriteString(" bash ")
	}
	b.WriteString(shellQuote(script.RemotePath))
	for _, arg := range args {
		b.WriteByte(' ')
		b.WriteString(shellQuote(arg))
	}
	return b.String()
}

func runScriptDisplay(script *RunScriptSpec, args []string) string {
	if script == nil {
		return strings.Join(args, " ")
	}
	words := append([]string{fmt.Sprintf("--script=%s", script.Source)}, args...)
	return strings.Join(readableShellWords(words), " ")
}

func runScriptRecordCommand(script *RunScriptSpec, args []string) []string {
	if script == nil {
		return args
	}
	if script.Source == "stdin" {
		return append([]string{"--script-stdin"}, args...)
	}
	return append([]string{"--script", script.Source}, args...)
}
