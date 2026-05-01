# Runner Bootstrap

Read when:

- changing cloud-init;
- debugging machines that never become SSH-ready;
- changing installed runner tools or readiness checks.

Each runner is an Ubuntu machine prepared by cloud-init. It does not need coordinator credentials.

Bootstrap creates:

- the `crabbox` user;
- SSH key-only access;
- SSH on port `2222`;
- `/work/crabbox`;
- shared package caches.

Bootstrap installs:

- Node 24;
- pnpm through corepack;
- Docker;
- Git;
- rsync;
- build-essential;
- jq;
- OpenSSH server.

Package setup uses retry wrappers for apt, NodeSource setup, and corepack. A machine should not pass readiness until `crabbox-ready` succeeds over SSH.

The CLI prefers the configured SSH port and can fall back to port 22 during early bootstrap. Long term, snapshots or provider images can replace slow cloud-init once the bootstrap contract is stable.

Related docs:

- [Providers](providers.md)
- [SSH keys](ssh-keys.md)
- [run command](../commands/run.md)
- [doctor command](../commands/doctor.md)
