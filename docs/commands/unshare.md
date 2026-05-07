# unshare

`crabbox unshare` removes sharing from an existing coordinator lease.

```sh
crabbox unshare --id blue-lobster --user friend@example.com
crabbox unshare --id blue-lobster --org
crabbox unshare --id blue-lobster --all
crabbox unshare blue-lobster --all --json
```

Use `--user` to remove individual users, `--org` to remove org-wide access, or
`--all` to clear every sharing rule. Only the lease owner, a `manage` share, or
an admin session can change sharing.

Flags:

```text
--id <lease-id-or-slug>
--user <email>
--org
--all
--json
```

Related docs:

- [share](share.md)
- [Auth and admin](../features/auth-admin.md)
- [Browser portal](../features/portal.md)
