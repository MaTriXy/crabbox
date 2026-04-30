package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Repo struct {
	Root string
	Name string
}

func findRepo() (Repo, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		wd, _ := os.Getwd()
		return Repo{Root: wd, Name: filepath.Base(wd)}, nil
	}
	root := strings.TrimSpace(string(out))
	return Repo{Root: root, Name: filepath.Base(root)}, nil
}

func defaultExcludes() []string {
	return []string{
		"node_modules",
		".turbo",
		".next",
		"dist",
		"dist-runtime",
		".cache",
		".pnpm-store",
		".git/lfs",
		".git/logs",
		".git/rr-cache",
		".git/worktrees",
	}
}

func allowedEnv() map[string]string {
	out := map[string]string{}
	for _, env := range os.Environ() {
		k, v, ok := strings.Cut(env, "=")
		if !ok {
			continue
		}
		if strings.HasPrefix(k, "OPENCLAW_") || k == "NODE_OPTIONS" || k == "CI" {
			out[k] = v
		}
	}
	return out
}
