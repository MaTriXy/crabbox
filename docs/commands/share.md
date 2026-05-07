# share

`crabbox share` grants access to an existing coordinator lease.

```sh
crabbox share --id blue-lobster --user friend@example.com
crabbox share --id blue-lobster --user friend@example.com --role manage
crabbox share --id blue-lobster --org
crabbox share --id blue-lobster --org --role manage
crabbox share --id blue-lobster --list
crabbox share blue-lobster --list --json
```

Roles:

```text
use     see the lease and use visible portal bridges such as WebVNC/code
manage  use access plus changing sharing and stopping the lease
```

`--org` shares with authenticated users whose org matches the lease org.
`--user` is repeatable and stores normalized lowercase email addresses.

SSH-based commands still require a local private key accepted by the runner.
Sharing grants coordinator and portal access; it does not copy SSH private keys
between people.

Flags:

```text
--id <lease-id-or-slug>
--user <email>
--org
--role use|manage
--list
--json
```

Related docs:

- [unshare](unshare.md)
- [Auth and admin](../features/auth-admin.md)
- [Browser portal](../features/portal.md)
