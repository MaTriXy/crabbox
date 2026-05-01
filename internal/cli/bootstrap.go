package cli

import "fmt"

func cloudInit(cfg Config, publicKey string) string {
	return fmt.Sprintf(`#cloud-config
package_update: false
package_upgrade: false
users:
  - name: %[1]s
    groups: sudo
    shell: /bin/bash
    sudo: ['ALL=(ALL) NOPASSWD:ALL']
    ssh_authorized_keys:
      - %[2]s
write_files:
  - path: /etc/ssh/sshd_config.d/99-crabbox-port.conf
    permissions: '0644'
    content: |
      Port 22
      Port %[4]s
      PasswordAuthentication no
  - path: /usr/local/bin/crabbox-ready
    permissions: '0755'
    content: |
      #!/usr/bin/env bash
      set -euo pipefail
      node --version
      pnpm --version
      git --version
      rsync --version >/dev/null
      docker --version
runcmd:
  - |
    bash -euxo pipefail <<'BOOT'
    export DEBIAN_FRONTEND=noninteractive
    cat >/etc/apt/apt.conf.d/80-crabbox-retries <<'APT'
    Acquire::Retries "8";
    Acquire::http::Timeout "30";
    Acquire::https::Timeout "30";
    APT
    retry() {
      n=1
      until "$@"; do
        if [ "$n" -ge 8 ]; then
          return 1
        fi
        sleep $((n * 5))
        n=$((n + 1))
      done
    }
    retry apt-get update
    retry apt-get install -y --no-install-recommends openssh-server ca-certificates curl git rsync build-essential docker.io jq
    mkdir -p %[3]s /var/cache/crabbox/pnpm /var/cache/crabbox/npm
    chown -R %[1]s:%[1]s %[3]s /var/cache/crabbox
    systemctl enable --now ssh
    systemctl restart ssh
    systemctl enable --now docker
    usermod -aG docker %[1]s
    retry bash -c 'curl -fsSL https://deb.nodesource.com/setup_24.x | bash -'
    retry apt-get install -y nodejs
    corepack enable
    retry corepack prepare pnpm@10.33.2 --activate
    sudo -u %[1]s bash -lc 'pnpm config set store-dir /var/cache/crabbox/pnpm'
    crabbox-ready
    BOOT
`, cfg.SSHUser, publicKey, cfg.WorkRoot, cfg.SSHPort)
}
