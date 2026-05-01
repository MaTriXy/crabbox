import type { LeaseConfig } from "./config";

export function cloudInit(config: LeaseConfig): string {
  return `#cloud-config
package_update: false
package_upgrade: false
users:
  - name: ${config.sshUser}
    groups: sudo
    shell: /bin/bash
    sudo: ['ALL=(ALL) NOPASSWD:ALL']
    ssh_authorized_keys:
      - ${config.sshPublicKey}
write_files:
  - path: /etc/ssh/sshd_config.d/99-crabbox-port.conf
    permissions: '0644'
    content: |
      Port 22
      Port ${config.sshPort}
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
    mkdir -p ${config.workRoot} /var/cache/crabbox/pnpm /var/cache/crabbox/npm
    chown -R ${config.sshUser}:${config.sshUser} ${config.workRoot} /var/cache/crabbox
    systemctl enable --now ssh
    systemctl restart ssh
    systemctl enable --now docker
    usermod -aG docker ${config.sshUser}
    retry bash -c 'curl -fsSL https://deb.nodesource.com/setup_24.x | bash -'
    retry apt-get install -y nodejs
    corepack enable
    retry corepack prepare pnpm@10.33.2 --activate
    sudo -u ${config.sshUser} bash -lc 'pnpm config set store-dir /var/cache/crabbox/pnpm'
    crabbox-ready
    BOOT
`;
}
