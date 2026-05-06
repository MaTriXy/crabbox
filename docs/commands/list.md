# list

`crabbox list` shows current Crabbox machines.

```sh
crabbox list
crabbox list --provider aws
crabbox list --provider ssh --target macos --static-host mac-studio.local
crabbox list --provider blacksmith-testbox
crabbox list --provider daytona
crabbox list --provider islo
crabbox list --json
```

`crabbox pool list` remains as a compatibility alias.

In `provider=ssh` mode this prints the configured static target.

In `blacksmith-testbox` mode this reads `blacksmith testbox list` and renders the
same Crabbox list shape as other providers. `--json` keeps the compatibility
shape parsed from the Blacksmith table: id, status, repo, workflow, job, ref,
and created time when the upstream table exposes those columns.

In `daytona` and `islo` modes, rendering is core-owned: human output and `--json`
use the normalized Crabbox lease view.

Flags:

```text
--provider hetzner|aws|ssh|blacksmith-testbox|daytona|islo
--target linux|macos|windows
--windows-mode normal|wsl2
--static-host <host>
--static-user <user>
--static-port <port>
--static-work-root <path>
--json
```
