# Browser Portal

Read when:

- using the web UI to inspect leases or runs;
- changing portal pages or page-level routes;
- deciding whether a feature should land in the CLI, the API, or the portal.

The browser portal is a small server-rendered web UI hosted by the same
Cloudflare Worker that backs the Crabbox API. It is not a separate frontend
or single-page app: every page is HTML rendered by the Worker, with light
client-side JavaScript only for filtering, sorting, and clipboard copy.

## URL Map

```text
GET  /portal
GET  /portal/leases/{id-or-slug}
POST /portal/leases/{id-or-slug}/release
GET  /portal/leases/{id-or-slug}/vnc
GET  /portal/leases/{id-or-slug}/code/
GET  /portal/runs/{run-id}
GET  /portal/runs/{run-id}/logs
GET  /portal/runs/{run-id}/events
GET  /portal/runners/{provider}/{runner-id}
```

Portal authentication uses a browser session cookie minted after a successful
GitHub login through the same OAuth flow as `crabbox login`. The cookie
carries owner/org claims; the Worker scopes every page to that identity. Raw
Cloudflare Access headers are not trusted - only a verified Access JWT email
can become the portal owner.

## Lease Index `/portal`

The index renders a searchable, paginated, sortable lease grid. Columns
include compact provider/target badges, icon-only access capabilities (SSH,
VNC, code, browser), relative time cells, dense rows, and sticky column
headers. Filters at the top of the page select active, ended, provider,
target, or all.

Default view rules:

- Defaults to active leases when any are active.
- Falls back to all visible leases when the active list is empty.
- Normal browser sessions see only their own owner/org leases.
- Admin sessions also see non-owned runner leases. `mine` and `system`
  filters distinguish personal leases from external runners (Blacksmith
  Testboxes synced from CLI list output) so external rows do not leak to
  normal users.

External runner rows render in the same grid as muted, disabled rows. They
include status/provider filters, inferred GitHub Actions run/workflow links,
status badges, `stuck` markers for long-queued or long-running Actions
owners, a copyable local stop command, and stale markers when the next
runner sync no longer sees a previously visible runner. Clicking an
external runner opens `/portal/runners/{provider}/{runner-id}`, a
visibility-only detail page.

## Lease Detail `/portal/leases/{id-or-slug}`

The lease detail page shows:

- compact provider/target badges and the lease state pill;
- bridge status for the WebVNC, code-server, and mediated egress bridges,
  including host/client connection state for an active egress session;
- the latest Linux telemetry sample as gauges, with sparklines when multiple
  samples are present;
- stale-telemetry, high-load, high-memory, and high-disk status pills when
  thresholds are exceeded;
- an access panel with copy-to-clipboard commands for `crabbox ssh`,
  `crabbox run`, `crabbox webvnc`, `crabbox code`, and (when an egress
  session is active) `crabbox egress status` / `crabbox egress stop`;
- a viewport-fitted "recent runs" grid with state filters;
- a stop action when the lease is releasable.

`/portal/leases/{id-or-slug}/vnc` and `/portal/leases/{id-or-slug}/code/`
are bridges, not portal pages. They proxy WebSocket and HTTP traffic to the
matching capability on the lease so a user does not need an SSH tunnel to
open the desktop or editor. The mediated egress bridge has its own
ticketed websocket route under `/v1/leases/{id-or-slug}/egress/...` rather
than a portal path, because egress is operator-driven and never opens an
HTML view. See [Interactive desktop and VNC](interactive-desktop-vnc.md),
[code command](../commands/code.md), and [Mediated egress](egress.md).

All bridge tickets travel as `Authorization: Bearer ...` headers on the
agent websocket upgrade, with a `?ticket=` query string fallback for older
CLIs. The portal never echoes ticket values back to the browser.

## Run Detail `/portal/runs/{run-id}`

Run detail mirrors the `/v1/runs/...` resources but uses the browser session
cookie, so users can inspect logs and events without copying a bearer token
into the browser. The page renders:

- the command, owner, lease, provider metadata, and exit status;
- a JUnit summary when the run attached results;
- a searchable, paginated event table with event-type filters;
- a copyable retained log tail;
- bounded load, memory, and disk trend lines for longer Linux runs that
  attached mid-run telemetry samples.

`/portal/runs/{run-id}/logs` returns the retained log as plain text.
`/portal/runs/{run-id}/events` returns the events as JSON. Both stay raw on
purpose so they are easy to copy or pipe.

## Runner Detail `/portal/runners/{provider}/{runner-id}`

External runner detail is visibility-only. It shows:

- owner/org;
- inferred GitHub Actions ownership (workflow, run id, status);
- lifecycle timestamps;
- boundary notes that explain Crabbox cannot stop or release the runner;
- a copyable local stop command for the operator's terminal.

External runners do not heartbeat through Crabbox and do not participate in
Crabbox lease expiry, cleanup, or cost accounting. The detail page exists so
operators have a single URL to share when an external runner is stuck.

## Authentication And Scope

```text
session  authenticated GitHub user (owner/org embedded)
admin    portal sessions with the admin token role
```

Per-route scope rules:

- Lease index, lease detail, run detail: own leases/runs only.
- Admin filters and external runner visibility: admin sessions only.
- VNC and code bridges: only when the lease has the matching capability and
  the session owns the lease.

Tokens for `/v1/...` API calls are separate. The portal never echoes a
bearer token back to the browser.

## Why Server-Rendered

The portal is intentionally a thin server-rendered surface, not a SPA:

- the Worker already owns lease and run data; rendering at the edge avoids a
  separate API/UI deployment;
- pages stay copy-pasteable - URLs deep-link to a specific lease or run;
- there is no build step, no JavaScript framework, and no offline session
  management to maintain;
- the portal cannot drift from the API because both serve the same Durable
  Object state.

Adding a portal feature usually means a new render in `worker/src/portal.ts`,
a new endpoint in `worker/src/fleet.ts`, and a doc update here.

Related docs:

- [Coordinator](coordinator.md)
- [Broker auth and routing](broker-auth-routing.md)
- [History and logs](history-logs.md)
- [Telemetry](telemetry.md)
- [Interactive desktop and VNC](interactive-desktop-vnc.md)
- [Source map](../source-map.md)
