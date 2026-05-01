# usage

`crabbox usage` shows orchestrator usage and estimated cost.

Usage is the command page for cost visibility. Keep command-specific behavior here; keep fleet policy and provider internals in [../orchestrator.md](../orchestrator.md) and [../features/cost-usage.md](../features/cost-usage.md).

```sh
crabbox usage
crabbox usage --scope org --org openclaw
crabbox usage --scope user --user peter@example.com --month 2026-05
crabbox usage --scope all --json
```

Usage requires a configured coordinator. Direct-provider mode has no central history to query.

Lease ownership comes from Cloudflare Access when available. In bearer-token mode, the CLI sends `CRABBOX_OWNER`, Git email env, or local `git config user.email`; set `CRABBOX_ORG` to group leases under an org.

## Scopes

Scopes:

```text
user    one owner email; default
org     one organization
all     whole fleet
```

## Flags

Flags:

```text
--scope user|org|all
--user <email>
--org <name>
--month YYYY-MM
--json
```

## Output

Human output prints one total row, then group rows when present:

```text
usage month=2026-05 scope=user user=steipete@gmail.com org=openclaw
total leases=2 active=0 runtime=12m41s estimated=$0.13 reserved=$4.57
owners:
  steipete@gmail.com       leases=2   active=0   runtime=12m41s    estimated=$0.13     reserved=$4.57
limits:
  active leases: fleet=off user=off org=off
  monthly usd:   fleet=off user=off org=off
```

`--json` returns the same summary and limit data for scripts:

```json
{
  "usage": {
    "month": "2026-05",
    "scope": "all",
    "leases": 6,
    "activeLeases": 0,
    "runtimeSeconds": 13551,
    "estimatedUSD": 0.13,
    "reservedUSD": 4.57,
    "byOwner": [],
    "byOrg": [],
    "byProvider": [],
    "byServerType": []
  },
  "limits": {
    "maxActiveLeases": 0,
    "maxActiveLeasesPerOwner": 0,
    "maxActiveLeasesPerOrg": 0,
    "maxMonthlyUSD": 0,
    "maxMonthlyUSDPerOwner": 0,
    "maxMonthlyUSDPerOrg": 0
  }
}
```

## Cost Estimates

Cost values are estimates for compute leases, not provider invoice reconciliation. They do not yet fully model provider extras such as static public IP charges, egress, storage, snapshots, taxes, credits, or discounts.

The coordinator chooses the hourly rate in this order:

```text
1. CRABBOX_COST_RATES_JSON explicit override.
2. Provider live pricing:
   - AWS: EC2 Spot price history for the requested instance type.
   - Hetzner: Cloud server-type hourly price for the requested location.
3. Built-in static fallback rates.
```

Explicit rates are useful for budget policy or conservative accounting. Example:

```sh
export CRABBOX_COST_RATES_JSON='{
  "aws": {
    "c7a.48xlarge": 2.25
  },
  "hetzner": {
    "ccx63": 0.44
  }
}'
```

Hetzner prices are returned in EUR. Crabbox converts them to USD with `CRABBOX_EUR_TO_USD`, default `1.08`.

## Estimated vs Reserved

`estimatedUSD` is elapsed runtime cost for leases in the selected month.

`reservedUSD` is worst-case reserved cost based on each lease TTL. The coordinator uses reserved cost before provisioning so monthly spend guardrails can reject a lease before it creates a machine.

## Limits

The `limits` block mirrors active coordinator guardrails:

```text
CRABBOX_MAX_ACTIVE_LEASES
CRABBOX_MAX_ACTIVE_LEASES_PER_OWNER
CRABBOX_MAX_ACTIVE_LEASES_PER_ORG
CRABBOX_MAX_MONTHLY_USD
CRABBOX_MAX_MONTHLY_USD_PER_OWNER
CRABBOX_MAX_MONTHLY_USD_PER_ORG
```

`0` means off and prints as `off`.
