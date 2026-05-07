# egress

`crabbox egress` bridges lease-local browser or app traffic through the machine
running the egress host agent.

```sh
crabbox egress start --id blue-lobster --profile discord
crabbox egress start --id blue-lobster --profile discord --daemon
crabbox desktop launch --id blue-lobster --browser --url https://discord.com/login --egress discord
crabbox egress status --id blue-lobster
crabbox egress stop --id blue-lobster
```

## How It Works

`egress start` installs a short-lived egress client helper on the lease, starts
a loopback HTTP proxy such as `127.0.0.1:3128`, then runs a local host bridge on
the operator machine. Both sides connect outbound to the coordinator with
one-use tickets. The coordinator pairs the two WebSockets and forwards
multiplexed proxy messages; it does not open internet connections itself.

The browser/app data path is:

```text
Chrome in lease
  -> lease 127.0.0.1:3128
  -> coordinator Durable Object
  -> local crabbox egress host process
  -> internet from the operator machine
```

`desktop launch --egress <profile>` passes the lease-local proxy to Chrome as:

```text
--proxy-server=http://127.0.0.1:3128
```

The portal lease detail page shows the active egress session, host/client
connection state, and copyable `egress status` / `egress stop` commands. It
does not expose tickets or raw proxy URLs.

## Subcommands

```text
start    Start a remote lease proxy and local host bridge
host     Run only the local egress host bridge
client   Run only the lease-side proxy bridge
status   Show coordinator bridge status
stop     Stop the local host daemon and remote lease client
```

Use `host` and `client` directly when debugging tickets, custom tunnels, or a
manually installed helper.

## Profiles And Allowlist

The host side refuses to become an open proxy. Use a built-in profile or an
explicit allowlist:

```sh
crabbox egress start --id blue-lobster --profile discord
crabbox egress start --id blue-lobster --allow example.com,*.example.com
```

Built-in profiles:

- `discord`: `discord.com`, `*.discord.com`, `discordcdn.com`,
  `*.discordcdn.com`, `hcaptcha.com`, `*.hcaptcha.com`
- `slack`: `slack.com`, `*.slack.com`, `slack-edge.com`, `*.slack-edge.com`

Wildcard entries match the named domain and subdomains.

## Flags

Common:

```text
--id <lease-id-or-slug>
--provider hetzner|aws
--profile <name>
--allow <comma-separated-host-patterns>
```

`start`:

```text
--listen 127.0.0.1:3128
--daemon
--target linux
--network auto|tailscale|public
```

`host` and `client` debugging:

```text
--coordinator <url>
--ticket <ticket>
--session <session-id>
```

## Limitations

- The shipped path is per-app/per-process egress, not full VM routing.
- `egress start` supports coordinator-backed Linux SSH leases.
- `egress start` refuses non-Linux targets until target-specific remote helper
  install/start commands exist.
- `egress start` does not install Cloudflare Access service-token credentials
  on the remote lease. If Access credentials are configured locally, use a
  public coordinator route, or run `egress client` manually only when it is safe
  to provide the required access headers.
- The first implementation uses JSON/base64 bridge frames. That is good enough
  for browser QA but can be optimized with binary frames later.

## Troubleshooting

`egress host requires --profile or --allow`

The host bridge will not start as an open proxy. Pick a profile or pass an
explicit allowlist.

`remote egress client did not listen`

Inspect the remote helper log:

```sh
crabbox ssh --id blue-lobster
cat /tmp/crabbox-egress-client.log
```

`desktop launch --egress currently requires --browser`

The automatic proxy flag is wired for browser launches. For custom apps, pass
the app's proxy flag yourself or use the lease-local proxy address printed by
`egress start`.
