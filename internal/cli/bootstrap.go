package cli

import "fmt"

func cloudInit(cfg Config, publicKey string) string {
	return fmt.Sprintf(`#cloud-config
package_update: true
package_upgrade: false
users:
  - name: %[1]s
    groups: sudo
    shell: /bin/bash
    sudo: ['ALL=(ALL) NOPASSWD:ALL']
    ssh_authorized_keys:
      - %[2]s
packages:
  - openssh-server
  - ca-certificates
  - curl
  - git
  - rsync
  - build-essential
  - docker.io
  - jq
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
  - mkdir -p %[3]s /var/cache/crabbox/pnpm /var/cache/crabbox/npm
  - chown -R %[1]s:%[1]s %[3]s /var/cache/crabbox
  - systemctl enable --now ssh
  - systemctl restart ssh
  - systemctl enable --now docker
  - usermod -aG docker %[1]s
  - curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
  - apt-get install -y nodejs
  - corepack enable
  - corepack prepare pnpm@10.33.2 --activate
  - sudo -u %[1]s bash -lc 'pnpm config set store-dir /var/cache/crabbox/pnpm'
  - crabbox-ready
`, cfg.SSHUser, publicKey, cfg.WorkRoot, cfg.SSHPort)
}
