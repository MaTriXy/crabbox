# SSH Keys

Read when:

- changing local key storage;
- debugging SSH auth;
- changing provider key-pair cleanup.

Crabbox creates a fresh SSH key per lease by default. This avoids sharing a long-lived personal key with every runner and gives the provider layer a resource name it can clean up.

Local key storage is under the Crabbox user config directory, outside the repository:

```text
macOS: ~/Library/Application Support/crabbox/keys/<lease>/
Linux: ~/.config/crabbox/keys/<lease>/
```

The CLI sends only the public key to the coordinator. The Worker imports or reuses that public key in the provider:

- Hetzner SSH key;
- AWS EC2 key pair.

When a coordinator returns a different final lease ID than the provisional CLI ID, the CLI moves the local key directory to the final ID so later `status`, `ssh`, `run --id`, and `stop` commands can reuse it.

Provider-side delete paths remove per-lease cloud keys/key pairs when machines are deleted. Explicit `CRABBOX_SSH_KEY` remains supported, but `doctor` only validates it when set.

Related docs:

- [Security](../security.md)
- [Runner bootstrap](runner-bootstrap.md)
- [ssh command](../commands/ssh.md)
- [doctor command](../commands/doctor.md)
