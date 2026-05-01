# ssh

`crabbox ssh` prints the SSH command for a lease.

```sh
crabbox ssh --id blue-lobster
```

The output includes the per-lease private key path when Crabbox created one. Printing an SSH command touches coordinator leases because it signals intended manual use.

Flags:

```text
--id <lease-id-or-slug>
--provider hetzner|aws
--reclaim
```

`ssh` touches the lease and validates the local repo claim. Use `--reclaim` when intentionally taking over a lease from another repo.
