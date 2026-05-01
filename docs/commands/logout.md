# logout

`crabbox logout` removes the stored broker token from user config.

```sh
crabbox logout
crabbox logout --json
```

The broker URL and provider are left in place so a later `crabbox login` or `crabbox login --token-stdin` can reuse them.

Related docs:

- [login](login.md)
- [whoami](whoami.md)
