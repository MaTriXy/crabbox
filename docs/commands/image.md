# image

`crabbox image` contains trusted operator controls for AWS runner images.

```sh
crabbox image create --id cbx_... --name openclaw-crabbox-20260501-1246 --wait
crabbox image promote ami-...
```

Image commands require a configured coordinator and shared-token admin auth.
They are intentionally not available to normal GitHub browser-login users.

## create

Create an AWS AMI from an active AWS lease.

Flags:

```text
--id <cbx_id>        source lease; must be a canonical AWS lease ID
--name <name>        AMI name
--wait               poll until the AMI is available
--wait-timeout <d>   default 45m
--no-reboot          default true
--json               print JSON
```

The source lease must still be active in the coordinator. The Worker calls AWS
`CreateImage` from the backing instance ID and tags the image as Crabbox-owned.

## promote

Promote an available AMI as the coordinator's default AWS image:

```sh
crabbox image promote ami-1234567890abcdef0
```

Future brokered AWS leases use the promoted image when the request does not set
an explicit `awsAMI` or `CRABBOX_AWS_AMI` override. Promotion stores coordinator
metadata only; it does not copy or modify the AMI.

Related docs:

- [Infrastructure](../infrastructure.md)
- [Runner bootstrap](../features/runner-bootstrap.md)
