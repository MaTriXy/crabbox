package cli

import (
	"strings"
	"testing"
)

func TestCloudInitUsesRetryingBootstrap(t *testing.T) {
	got := cloudInit(baseConfig(), "ssh-ed25519 test")
	for _, want := range []string{
		"package_update: false",
		"bash -euxo pipefail <<'BOOT'",
		"Acquire::Retries \"8\";",
		"retry apt-get update",
		"retry apt-get install -y --no-install-recommends openssh-server ca-certificates curl git rsync build-essential docker.io jq",
		"https://deb.nodesource.com/setup_24.x",
		"retry corepack prepare pnpm@10.33.2 --activate",
		"docker --version",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("cloudInit() missing %q", want)
		}
	}
	if strings.Contains(got, "\npackages:\n") {
		t.Fatal("cloudInit() must not use cloud-init's one-shot packages module")
	}
}
