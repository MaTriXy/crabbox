import type { LeaseRecord, RunEventRecord, RunRecord } from "./types";

const novncModuleURL = "/portal/assets/novnc/rfb.js";

export interface PortalLeaseBridgeStatus {
  webVNCBridgeConnected: boolean;
  webVNCViewerConnected: boolean;
  codeBridgeConnected: boolean;
}

export function portalHome(leases: LeaseRecord[], request: Request): Response {
  const active = leases.filter((lease) => lease.state === "active");
  const rows = active.length
    ? active.map((lease) => leaseRow(lease)).join("")
    : `<tr><td colspan="7" class="empty">no active leases</td></tr>`;
  return html(
    "Crabbox Portal",
    `<main>
      <header class="top">
        <div>
          <h1>Crabbox</h1>
          <p>${escapeHTML(new URL(request.url).host)}</p>
        </div>
        <a class="button secondary" href="/portal/logout">log out</a>
      </header>
      <section class="panel">
        <div class="section-head">
          <h2>leases</h2>
          <span>${active.length} active</span>
        </div>
        <table>
          <thead>
            <tr>
              <th>lease</th>
              <th>provider</th>
              <th>target</th>
              <th>class</th>
              <th>access</th>
              <th>expires</th>
              <th></th>
            </tr>
          </thead>
          <tbody>${rows}</tbody>
        </table>
      </section>
    </main>`,
  );
}

export function portalLeaseDetail(
  lease: LeaseRecord,
  runs: RunRecord[],
  bridgeStatus: PortalLeaseBridgeStatus,
): Response {
  const slug = lease.slug || lease.id;
  const runRows = runs.length
    ? runs.map((run) => runRow(run)).join("")
    : `<tr><td colspan="7" class="empty">no recorded runs for this lease</td></tr>`;
  const vncAction = lease.desktop
    ? `<a class="button" href="/portal/leases/${encodeURIComponent(lease.id)}/vnc">open VNC</a>`
    : `<span class="muted">no desktop</span>`;
  const codeAction = lease.code
    ? `<a class="button" href="/portal/leases/${encodeURIComponent(lease.id)}/code/">open code</a>`
    : `<span class="muted">no code</span>`;
  const commands = [
    commandBlock("shell", `crabbox ssh --id ${shellArg(slug)}`),
    commandBlock("run", `crabbox run --id ${shellArg(slug)} -- <command>`),
    lease.desktop ? commandBlock("WebVNC bridge", webVNCBridgeCommand(lease)) : "",
    lease.code ? commandBlock("code bridge", codeBridgeCommand(lease)) : "",
  ]
    .filter(Boolean)
    .join("");
  return html(
    `${slug} lease`,
    `<main>
      <header class="top">
        <div>
          <h1>${escapeHTML(slug)}</h1>
          <p>${escapeHTML(lease.provider)} ${escapeHTML(lease.target)} lease <span class="mono">${escapeHTML(lease.id)}</span></p>
        </div>
        <div class="vnc-actions">
          <a class="button secondary" href="/portal">leases</a>
          <a class="button secondary" href="/portal/logout">log out</a>
        </div>
      </header>
      <section class="detail-grid">
        <div class="panel detail-card">
          <div class="section-head">
            <h2>status</h2>
            <span class="pill" data-state="${escapeHTML(lease.state)}">${escapeHTML(lease.state)}</span>
          </div>
          <dl class="meta-grid">
            ${metaRow("provider", lease.provider)}
            ${metaRow("target", lease.windowsMode ? `${lease.target} / ${lease.windowsMode}` : lease.target)}
            ${metaRow("class", lease.class)}
            ${metaRow("host", lease.host || "pending")}
            ${metaRow("ssh", lease.sshPort ? `${lease.sshUser || "crabbox"}@${lease.host || "host"}:${lease.sshPort}` : "pending")}
            ${metaRow("work root", lease.workRoot || "pending")}
            ${metaRow("expires", shortTime(lease.expiresAt))}
          </dl>
          <form method="post" action="/portal/leases/${encodeURIComponent(lease.id)}/release" class="stop-form">
            <button class="button danger" type="submit">stop lease</button>
          </form>
        </div>
        <div class="panel detail-card">
          <div class="section-head">
            <h2>access</h2>
            <span>${lease.desktop || lease.code ? "bridges" : "ssh only"}</span>
          </div>
          <div class="bridge-grid">
            ${bridgeRow("WebVNC", lease.desktop === true, bridgeStatus.webVNCBridgeConnected, bridgeStatus.webVNCViewerConnected, vncAction)}
            ${bridgeRow("code", lease.code === true, bridgeStatus.codeBridgeConnected, false, codeAction)}
          </div>
        </div>
      </section>
      <section class="panel">
        <div class="section-head">
          <h2>commands</h2>
          <span>copy locally</span>
        </div>
        <div class="commands">${commands}</div>
      </section>
      <section class="panel">
        <div class="section-head">
          <h2>recent runs</h2>
          <span>${runs.length}</span>
        </div>
        <table class="run-table">
          <thead>
            <tr>
              <th>run</th>
              <th>state</th>
              <th>phase</th>
              <th>started</th>
              <th>duration</th>
              <th>log</th>
              <th></th>
            </tr>
          </thead>
          <tbody>${runRows}</tbody>
        </table>
      </section>
    </main>`,
  );
}

export function portalRunDetail(
  run: RunRecord,
  events: RunEventRecord[],
  logTail: string,
): Response {
  const stateTone = run.state === "succeeded" ? "ok" : run.state === "failed" ? "bad" : "warn";
  const eventRows = events.length
    ? events.map((event) => eventRow(event)).join("")
    : `<tr><td colspan="5" class="empty">no events recorded</td></tr>`;
  const failureRows = run.results?.failed.length
    ? run.results.failed
        .slice(0, 8)
        .map(
          (failure) => `<li>
            <strong>${escapeHTML(failure.name)}</strong>
            <small>${escapeHTML([failure.suite, failure.file].filter(Boolean).join(" / "))}</small>
            ${failure.message ? `<p>${escapeHTML(truncate(failure.message, 240))}</p>` : ""}
          </li>`,
        )
        .join("")
    : "";
  const logBlock = logTail
    ? `<pre class="log-preview">${escapeHTML(logTail)}</pre>`
    : `<p class="empty">no retained log output</p>`;
  return html(
    `${run.id} run`,
    `<main>
      <header class="top">
        <div>
          <h1>${escapeHTML(run.id)}</h1>
          <p>${escapeHTML(run.slug || run.leaseID)} <span class="mono">${escapeHTML(run.command.join(" "))}</span></p>
        </div>
        <div class="vnc-actions">
          <a class="button secondary" href="/portal/leases/${encodeURIComponent(run.leaseID)}">lease</a>
          <a class="button secondary" href="/portal">leases</a>
          <a class="button secondary" href="/portal/logout">log out</a>
        </div>
      </header>
      <section class="detail-grid">
        <div class="panel detail-card">
          <div class="section-head">
            <h2>run</h2>
            <span class="pill" data-tone="${stateTone}">${escapeHTML(run.state)}</span>
          </div>
          <dl class="meta-grid">
            ${metaRow("lease", run.slug ? `${run.slug} / ${run.leaseID}` : run.leaseID)}
            ${metaRow("provider", run.provider)}
            ${metaRow("target", run.windowsMode ? `${run.target || "linux"} / ${run.windowsMode}` : run.target || "linux")}
            ${metaRow("class", run.class)}
            ${metaRow("server type", run.serverType)}
            ${metaRow("phase", run.phase || run.state)}
            ${metaRow("exit", formatExitCode(run.exitCode))}
            ${metaRow("started", shortTime(run.startedAt))}
            ${metaRow("duration", formatDuration(run.durationMs))}
            ${metaRow("log", run.logBytes > 0 ? formatBytes(run.logBytes) : "empty")}
          </dl>
        </div>
        <div class="panel detail-card">
          <div class="section-head">
            <h2>artifacts</h2>
            <span>${run.results ? "junit" : "logs"}</span>
          </div>
          <div class="run-artifacts">
            <a class="button" href="/portal/runs/${encodeURIComponent(run.id)}/logs">raw logs</a>
            <a class="button secondary" href="/portal/runs/${encodeURIComponent(run.id)}/events">events json</a>
            ${resultsSummary(run)}
          </div>
        </div>
      </section>
      <section class="panel">
        <div class="section-head">
          <h2>command</h2>
          <span>${escapeHTML(run.owner)}</span>
        </div>
        <div class="commands">${commandBlock("remote command", run.command.join(" "))}</div>
      </section>
      ${
        failureRows
          ? `<section class="panel">
              <div class="section-head">
                <h2>failures</h2>
                <span>${run.results?.failed.length ?? 0}</span>
              </div>
              <ul class="failure-list">${failureRows}</ul>
            </section>`
          : ""
      }
      <section class="panel">
        <div class="section-head">
          <h2>log tail</h2>
          <span>${run.logTruncated ? "truncated" : "retained"}</span>
        </div>
        ${logBlock}
      </section>
      <section class="panel">
        <div class="section-head">
          <h2>events</h2>
          <span>${events.length}</span>
        </div>
        <table class="event-table">
          <thead>
            <tr>
              <th>seq</th>
              <th>type</th>
              <th>phase</th>
              <th>time</th>
              <th>message</th>
            </tr>
          </thead>
          <tbody>${eventRows}</tbody>
        </table>
      </section>
    </main>`,
  );
}

export function portalVNC(lease: LeaseRecord): Response {
  const nonce = scriptNonce();
  const slug = lease.slug || lease.id;
  const title = `WebVNC ${slug}`;
  const wsPath = `/portal/leases/${encodeURIComponent(lease.id)}/vnc/viewer`;
  const statusPath = `/portal/leases/${encodeURIComponent(lease.id)}/vnc/status`;
  const bridgeCmd = webVNCBridgeCommand(lease);
  const fullscreenIcon = `<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 9V4h5"/><path d="M20 9V4h-5"/><path d="M4 15v5h5"/><path d="M20 15v5h-5"/></svg>`;
  const copyIcon = `<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15V5a2 2 0 0 1 2-2h10"/></svg>`;
  const reconnectIcon = `<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12a9 9 0 1 1-3-6.7"/><path d="M21 4v5h-5"/></svg>`;
  return html(
    title,
    `<main class="vnc-page">
      <header class="vnc-bar">
        <div class="vnc-meta">
          <h1>${escapeHTML(slug)}</h1>
          <p><span>${escapeHTML(lease.provider)}</span><span class="vnc-dot"></span><span>${escapeHTML(lease.target)}</span><span class="vnc-dot"></span><span class="vnc-id">${escapeHTML(lease.id)}</span></p>
        </div>
        <div class="vnc-actions">
          <span id="status" class="status-pill">waiting for bridge</span>
          <button id="vnc-reconnect" class="icon-btn" type="button" title="reconnect" aria-label="reconnect">${reconnectIcon}</button>
          <button id="vnc-fullscreen" class="icon-btn" type="button" title="fullscreen" aria-label="toggle fullscreen">${fullscreenIcon}</button>
          <a class="button secondary" href="/portal">leases</a>
          <a class="button secondary" href="/portal/logout">log out</a>
        </div>
      </header>
      <section id="screen" class="screen" aria-label="WebVNC display"></section>
      <footer class="vnc-bridge">
        <span class="vnc-bridge-label">bridge</span>
        <code id="vnc-bridge-cmd" class="vnc-bridge-cmd">${escapeHTML(bridgeCmd)}</code>
        <button id="vnc-copy" class="icon-btn" type="button" title="copy command" aria-label="copy bridge command">${copyIcon}</button>
      </footer>
    </main>
    <script type="module" nonce="${nonce}">
      import RFBModule from ${JSON.stringify(novncModuleURL)};
      const RFB = RFBModule.default || RFBModule;
      const status = document.getElementById("status");
      const screen = document.getElementById("screen");
      const wsURL = new URL(${JSON.stringify(wsPath)}, window.location.href);
      wsURL.protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
      const statusURL = new URL(${JSON.stringify(statusPath)}, window.location.href);
      const fragment = new URLSearchParams(window.location.hash.slice(1));
      const username = fragment.get("username") || "";
      const password = fragment.get("password") || "";
      const credentials = {};
      if (username) credentials.username = username;
      if (password) credentials.password = password;
      const options = Object.keys(credentials).length ? { credentials } : {};
      function setStatus(value, tone = "") {
        status.textContent = value;
        status.dataset.tone = tone;
      }
      let rfb;
      let retryTimer;
      let retryAttempt = 0;
      let connected = false;
      let stopped = false;
      function retryDelay() {
        return Math.min(5000, 500 * 2 ** retryAttempt);
      }
      async function bridgeState() {
        try {
          const response = await fetch(statusURL, { cache: "no-store" });
          return response.ok ? await response.json() : undefined;
        } catch {
          return undefined;
        }
      }
      function scheduleRetry(label) {
        if (stopped) return;
        const delay = retryDelay();
        retryAttempt += 1;
        setStatus(label + "; retrying in " + Math.ceil(delay / 1000) + "s", "warn");
        window.clearTimeout(retryTimer);
        retryTimer = window.setTimeout(connect, delay);
      }
      async function connect() {
        if (stopped) return;
        connected = false;
        screen.replaceChildren();
        try {
          const state = await bridgeState();
          if (state && !state.bridgeConnected) {
            scheduleRetry(state.message || "no bridge connected; run the bridge command below");
            return;
          }
          if (state?.viewerConnected) {
            setStatus("another viewer is connected; close stale tabs if this resets", "warn");
          } else {
            setStatus(retryAttempt ? "bridge connected; opening viewer" : "connecting");
          }
          rfb = new RFB(screen, wsURL.toString(), options);
          rfb.scaleViewport = true;
          rfb.resizeSession = false;
          rfb.viewOnly = false;
          rfb.addEventListener("connect", () => {
            connected = true;
            retryAttempt = 0;
            setStatus("connected", "ok");
          });
          rfb.addEventListener("disconnect", () => {
            scheduleRetry(connected ? "bridge disconnected" : "waiting for bridge");
          });
          rfb.addEventListener("credentialsrequired", (event) => {
            const types = event.detail?.types || ["password"];
            const values = {};
            if (types.includes("username")) {
              values.username = username || window.prompt("VNC username") || "";
            }
            if (types.includes("password")) {
              values.password = password || window.prompt("VNC password") || "";
            }
            rfb.sendCredentials(values);
          });
          rfb.addEventListener("securityfailure", () => {
            stopped = true;
            window.clearTimeout(retryTimer);
            setStatus("VNC authentication failed", "bad");
          });
        } catch (error) {
          scheduleRetry(error instanceof Error ? error.message : String(error));
        }
      }
      window.addEventListener("beforeunload", () => {
        stopped = true;
        window.clearTimeout(retryTimer);
        rfb?.disconnect();
      });
      const reconnectBtn = document.getElementById("vnc-reconnect");
      reconnectBtn?.addEventListener("click", () => {
        window.clearTimeout(retryTimer);
        retryAttempt = 0;
        stopped = false;
        try { rfb?.disconnect(); } catch (_) {}
        connect();
      });
      const fullscreenBtn = document.getElementById("vnc-fullscreen");
      fullscreenBtn?.addEventListener("click", () => {
        if (document.fullscreenElement) {
          document.exitFullscreen();
        } else {
          document.documentElement.requestFullscreen?.().catch(() => {});
        }
      });
      const copyBtn = document.getElementById("vnc-copy");
      const cmdEl = document.getElementById("vnc-bridge-cmd");
      let copyResetTimer;
      copyBtn?.addEventListener("click", async () => {
        const text = cmdEl?.textContent || "";
        try {
          await navigator.clipboard.writeText(text);
        } catch (_) {
          const range = document.createRange();
          if (cmdEl) {
            range.selectNodeContents(cmdEl);
            const sel = window.getSelection();
            sel?.removeAllRanges();
            sel?.addRange(range);
          }
        }
        copyBtn.dataset.state = "ok";
        window.clearTimeout(copyResetTimer);
        copyResetTimer = window.setTimeout(() => { delete copyBtn.dataset.state; }, 1200);
      });
      connect();
    </script>`,
    200,
    nonce,
  );
}

export function portalError(title: string, message: string, status = 400): Response {
  return html(
    title,
    `<main>
      <section class="panel error">
        <h1>${escapeHTML(title)}</h1>
        <p>${escapeHTML(message)}</p>
        <a class="button secondary" href="/portal">back to portal</a>
      </section>
    </main>`,
    status,
  );
}

export function portalCode(lease: LeaseRecord): Response {
  const slug = lease.slug || lease.id;
  const bridgeCmd = codeBridgeCommand(lease);
  return html(
    `Code ${slug}`,
    `<main>
      <header class="top">
        <div>
          <h1>${escapeHTML(slug)}</h1>
          <p>${escapeHTML(lease.provider)} code workspace</p>
        </div>
        <div class="vnc-actions">
          <a class="button secondary" href="/portal">leases</a>
          <a class="button secondary" href="/portal/logout">log out</a>
        </div>
      </header>
      <section class="panel error">
        <h2>code bridge</h2>
        <p class="muted">start the local bridge, then reload this page.</p>
        <code>${escapeHTML(bridgeCmd)}</code>
      </section>
    </main>`,
  );
}

export function codeBridgeCommand(lease: LeaseRecord): string {
  return ["crabbox", "code", "--id", lease.slug || lease.id, "--open"].map(shellArg).join(" ");
}

export function webVNCBridgeCommand(lease: LeaseRecord): string {
  const target = lease.target || "linux";
  const args = [
    "crabbox",
    "webvnc",
    "--provider",
    lease.provider,
    "--target",
    target,
    "--id",
    lease.slug || lease.id,
  ];
  if (target === "windows" && lease.windowsMode && lease.windowsMode !== "normal") {
    args.push("--windows-mode", lease.windowsMode);
  }
  args.push("--open");
  return args.map(shellArg).join(" ");
}

function shellArg(value: string): string {
  if (/^[A-Za-z0-9_./:@=-]+$/.test(value)) {
    return value;
  }
  return `'${value.replaceAll("'", "'\"'\"'")}'`;
}

function leaseRow(lease: LeaseRecord): string {
  const label = lease.slug || lease.id;
  const detailPath = `/portal/leases/${encodeURIComponent(lease.id)}`;
  const vnc = lease.desktop
    ? `<a class="button" href="/portal/leases/${encodeURIComponent(lease.id)}/vnc">open</a>`
    : `<span class="muted">no desktop</span>`;
  const code = lease.code
    ? `<a class="button secondary" href="/portal/leases/${encodeURIComponent(lease.id)}/code/">code</a>`
    : `<span class="muted">no code</span>`;
  return `<tr>
    <td><a class="lease-link" href="${detailPath}"><strong>${escapeHTML(label)}</strong><small>${escapeHTML(lease.id)}</small></a></td>
    <td>${escapeHTML(lease.provider)}</td>
    <td>${escapeHTML(lease.target)}</td>
    <td>${escapeHTML(lease.class)}</td>
    <td><div class="actions-cell">${vnc}${code}</div></td>
    <td>${escapeHTML(shortTime(lease.expiresAt))}</td>
    <td></td>
  </tr>`;
}

function runRow(run: RunRecord): string {
  const stateTone = run.state === "succeeded" ? "ok" : run.state === "failed" ? "bad" : "warn";
  const logLabel = run.logBytes > 0 ? formatBytes(run.logBytes) : "empty";
  return `<tr>
    <td><a class="lease-link" href="/portal/runs/${encodeURIComponent(run.id)}"><strong>${escapeHTML(run.id)}</strong><small>${escapeHTML(run.command.join(" "))}</small></a></td>
    <td><span class="pill" data-tone="${stateTone}">${escapeHTML(run.state)}</span></td>
    <td>${escapeHTML(run.phase || "-")}</td>
    <td>${escapeHTML(shortTime(run.startedAt))}</td>
    <td>${escapeHTML(formatDuration(run.durationMs))}</td>
    <td>${escapeHTML(logLabel)}</td>
    <td><div class="actions-cell"><a class="button secondary" href="/portal/runs/${encodeURIComponent(run.id)}/logs">logs</a><a class="button secondary" href="/portal/runs/${encodeURIComponent(run.id)}/events">events</a></div></td>
  </tr>`;
}

function eventRow(event: RunEventRecord): string {
  return `<tr>
    <td>${event.seq}</td>
    <td><strong>${escapeHTML(event.type)}</strong><small>${escapeHTML(event.stream || "")}</small></td>
    <td>${escapeHTML(event.phase || "-")}</td>
    <td>${escapeHTML(shortTime(event.createdAt))}</td>
    <td>${escapeHTML(truncate(event.message || event.data || "", 220))}</td>
  </tr>`;
}

function metaRow(label: string, value: string | undefined): string {
  return `<div><dt>${escapeHTML(label)}</dt><dd>${escapeHTML(value || "-")}</dd></div>`;
}

function bridgeRow(
  label: string,
  enabled: boolean,
  bridgeConnected: boolean,
  viewerConnected: boolean,
  action: string,
): string {
  const status = enabled
    ? bridgeConnected
      ? viewerConnected
        ? "viewer active"
        : "bridge ready"
      : "waiting for bridge"
    : "unavailable";
  const tone = enabled ? (bridgeConnected ? "ok" : "warn") : "";
  return `<div class="bridge-row">
    <div><strong>${escapeHTML(label)}</strong><small>${escapeHTML(status)}</small></div>
    <span class="pill" data-tone="${tone}">${escapeHTML(enabled ? (bridgeConnected ? "connected" : "waiting") : "off")}</span>
    ${action}
  </div>`;
}

function commandBlock(label: string, command: string): string {
  return `<div class="command-row"><div><small>${escapeHTML(label)}</small><code>${escapeHTML(command)}</code></div></div>`;
}

function resultsSummary(run: RunRecord): string {
  if (!run.results) {
    return `<p class="muted">no test result summary</p>`;
  }
  const result = run.results;
  return `<dl class="result-grid">
    ${metaRow("tests", String(result.tests))}
    ${metaRow("failures", String(result.failures))}
    ${metaRow("errors", String(result.errors))}
    ${metaRow("skipped", String(result.skipped))}
    ${metaRow("time", `${result.timeSeconds}s`)}
  </dl>`;
}

function html(title: string, body: string, status = 200, nonce = ""): Response {
  const scriptSource = nonce ? `'self' 'nonce-${nonce}'` : "'self'";
  return new Response(
    `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <meta name="color-scheme" content="dark">
  <meta name="theme-color" content="#0b0d0f">
  <title>${escapeHTML(title)}</title>
  <style>
    :root { color-scheme: dark; --bg:#0b0d0f; --fg:#f3f5f7; --muted:#9ca3af; --line:#262b31; --line-soft:#1d2126; --panel:#15181c; --panel-2:#0f1215; --accent:#38bdf8; --bad:#f87171; --warn:#fbbf24; --ok:#34d399; --mono: ui-monospace,SFMono-Regular,Menlo,Consolas,monospace; }
    * { box-sizing: border-box; }
    html { background:var(--bg); }
    body { margin:0; min-height:100vh; background:var(--bg); color:var(--fg); font:14px/1.45 ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif; }
    main { width:min(1180px, calc(100vw - 32px)); margin:0 auto; padding:24px 0; }
    h1,h2,p { margin:0; }
    h1 { font-size:22px; font-weight:700; }
    h2 { font-size:14px; text-transform:uppercase; color:var(--muted); }
    a { color:inherit; }
    form { margin:0; }
    button { font:inherit; }
    code { display:block; overflow:auto; padding:12px; border:1px solid var(--line); border-radius:6px; background:#0c0e10; color:#d1fae5; font-family:var(--mono); }
    table { width:100%; border-collapse:collapse; table-layout:fixed; }
    th,td { padding:12px; border-bottom:1px solid var(--line); text-align:left; vertical-align:middle; }
    th { color:var(--muted); font-weight:600; }
    td small { display:block; color:var(--muted); margin-top:2px; }
    .top { display:flex; justify-content:space-between; gap:16px; align-items:center; margin-bottom:20px; }
    .top p,.muted,.empty { color:var(--muted); }
    .panel { border:1px solid var(--line); border-radius:8px; background:var(--panel); overflow:hidden; }
    .section-head { display:flex; justify-content:space-between; align-items:center; padding:14px 16px; border-bottom:1px solid var(--line); }
    .button { display:inline-flex; align-items:center; justify-content:center; min-height:32px; padding:0 12px; border-radius:8px; background:var(--accent); color:#001018; text-decoration:none; font-weight:700; }
    .button.secondary { background:transparent; color:var(--fg); border:1px solid var(--line); font-weight:500; }
    .button.secondary:hover { background:#1b1f24; border-color:#3a4046; }
    .button.danger { border:1px solid color-mix(in srgb, var(--bad) 42%, var(--line)); background:color-mix(in srgb, var(--bad) 18%, transparent); color:#fecaca; cursor:pointer; }
    .lease-link { display:block; text-decoration:none; }
    .mono { font-family:var(--mono); }
    .detail-grid { display:grid; grid-template-columns:minmax(0,1.1fr) minmax(280px,0.9fr); gap:12px; margin-bottom:12px; }
    .detail-card { min-width:0; }
    .meta-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:0; margin:0; }
    .meta-grid div { padding:12px 14px; border-bottom:1px solid var(--line-soft); }
    .meta-grid dt { color:var(--muted); font-size:11px; text-transform:uppercase; margin-bottom:3px; }
    .meta-grid dd { margin:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
    .stop-form { padding:14px; }
    .bridge-grid { display:grid; gap:0; }
    .bridge-row { display:grid; grid-template-columns:minmax(0,1fr) auto auto; gap:10px; align-items:center; padding:14px; border-bottom:1px solid var(--line-soft); }
    .bridge-row small { display:block; color:var(--muted); margin-top:2px; }
    .run-artifacts { display:grid; gap:10px; padding:14px; }
    .result-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:0; margin:4px -14px -14px; border-top:1px solid var(--line-soft); }
    .result-grid div { padding:10px 14px; border-bottom:1px solid var(--line-soft); }
    .result-grid dt { color:var(--muted); font-size:11px; text-transform:uppercase; margin-bottom:3px; }
    .result-grid dd { margin:0; }
    .log-preview { margin:0; max-height:420px; overflow:auto; padding:14px; background:#080a0c; color:#d1fae5; border:0; border-radius:0; font-family:var(--mono); font-size:12px; line-height:1.5; white-space:pre-wrap; overflow-wrap:anywhere; }
    .failure-list { display:grid; gap:0; margin:0; padding:0; list-style:none; }
    .failure-list li { padding:14px 16px; border-bottom:1px solid var(--line-soft); }
    .failure-list small { display:block; color:var(--muted); margin-top:2px; }
    .failure-list p { margin-top:8px; color:#fecaca; }
    .pill { display:inline-flex; align-items:center; justify-content:center; min-height:24px; padding:0 8px; border-radius:999px; border:1px solid var(--line); color:var(--muted); background:var(--panel-2); font-size:12px; white-space:nowrap; }
    .pill[data-tone="ok"],.pill[data-state="active"] { color:var(--ok); border-color:color-mix(in srgb, var(--ok) 35%, var(--line)); }
    .pill[data-tone="warn"] { color:var(--warn); border-color:color-mix(in srgb, var(--warn) 35%, var(--line)); }
    .pill[data-tone="bad"],.pill[data-state="released"],.pill[data-state="expired"] { color:var(--bad); border-color:color-mix(in srgb, var(--bad) 45%, var(--line)); }
    .actions-cell { display:flex; align-items:center; gap:8px; flex-wrap:wrap; }
    .vnc-page { width:100vw; height:100vh; padding:10px 12px 10px; display:grid; grid-template-rows:auto 1fr auto; gap:10px; }
    .vnc-bar { display:flex; align-items:center; justify-content:space-between; gap:16px; min-height:44px; padding:0 4px; }
    .vnc-meta { display:flex; align-items:baseline; gap:12px; min-width:0; }
    .vnc-meta h1 { font-size:18px; font-weight:700; letter-spacing:-0.01em; white-space:nowrap; }
    .vnc-meta p { display:inline-flex; align-items:center; gap:8px; color:var(--muted); font-size:12px; min-width:0; overflow:hidden; }
    .vnc-meta .vnc-id { font-family:var(--mono); font-size:11px; opacity:0.85; }
    .vnc-meta .vnc-dot { width:3px; height:3px; border-radius:50%; background:#3a4046; flex-shrink:0; }
    .vnc-actions { display:flex; align-items:center; gap:8px; flex-shrink:0; }
    .status-pill { display:inline-flex; align-items:center; gap:8px; height:32px; padding:0 12px 0 11px; border-radius:8px; background:var(--panel-2); border:1px solid var(--line); font-size:12px; color:var(--muted); white-space:nowrap; transition:color 0.2s, border-color 0.2s; }
    .status-pill::before { content:""; width:8px; height:8px; border-radius:50%; background:currentColor; box-shadow:0 0 0 3px color-mix(in srgb, currentColor 18%, transparent); flex-shrink:0; }
    .status-pill[data-tone="ok"] { color:var(--ok); border-color:color-mix(in srgb, var(--ok) 35%, var(--line)); }
    .status-pill[data-tone="warn"] { color:var(--warn); border-color:color-mix(in srgb, var(--warn) 35%, var(--line)); }
    .status-pill[data-tone="bad"] { color:var(--bad); border-color:color-mix(in srgb, var(--bad) 45%, var(--line)); }
    .icon-btn { display:inline-flex; align-items:center; justify-content:center; width:32px; height:32px; padding:0; border-radius:8px; background:transparent; color:var(--fg); border:1px solid var(--line); cursor:pointer; transition:background 0.15s, border-color 0.15s, color 0.15s; }
    .icon-btn:hover { background:#1b1f24; border-color:#3a4046; }
    .icon-btn:active { background:#22272d; }
    .icon-btn[data-state="ok"] { color:var(--ok); border-color:color-mix(in srgb, var(--ok) 45%, var(--line)); }
    .screen { min-height:0; border:1px solid var(--line); border-radius:8px; background:var(--bg); overflow:hidden; box-shadow:inset 0 0 0 1px rgba(255,255,255,0.02); }
    .screen div { margin:0 auto; }
    .vnc-bridge { display:flex; align-items:center; gap:10px; padding:6px 10px; border:1px solid var(--line); border-radius:8px; background:var(--panel); }
    .vnc-bridge-label { font-size:10px; text-transform:uppercase; letter-spacing:0.08em; color:var(--muted); flex-shrink:0; padding-left:4px; }
    .vnc-bridge-cmd { display:block; flex:1; min-width:0; padding:6px 10px; border:none; border-radius:5px; background:transparent; color:#d1fae5; font-family:var(--mono); font-size:13px; overflow-x:auto; white-space:nowrap; }
    .commands { padding:12px; display:grid; gap:8px; }
    .command-row { display:grid; grid-template-columns:minmax(0,1fr); gap:8px; align-items:stretch; }
    .command-row small { display:block; color:var(--muted); margin-bottom:4px; text-transform:uppercase; font-size:11px; }
    .command-row code { min-width:0; }
    .error { margin-top:20vh; padding:24px; display:grid; gap:12px; }
    @media (max-width: 760px) {
      main { width:min(100vw - 20px, 1180px); padding:10px 0; }
      th:nth-child(4),td:nth-child(4),th:nth-child(6),td:nth-child(6){ display:none; }
      .detail-grid { grid-template-columns:1fr; }
      .meta-grid { grid-template-columns:1fr; }
      .result-grid { grid-template-columns:1fr; }
      .bridge-row { grid-template-columns:1fr; align-items:start; }
      .top{align-items:flex-start;}
      .vnc-bar { flex-wrap:wrap; gap:8px; min-height:0; padding:4px 0; }
      .vnc-meta { flex-wrap:wrap; gap:4px 10px; }
      .vnc-meta p .vnc-id { display:none; }
      .vnc-actions { gap:6px; }
      .vnc-actions .button { min-height:30px; padding:0 10px; }
      .vnc-bridge-label { display:none; }
    }
  </style>
</head>
<body>${body}</body>
</html>`,
    {
      status,
      headers: {
        "content-security-policy": [
          "default-src 'none'",
          "base-uri 'none'",
          "connect-src 'self' ws: wss:",
          "frame-ancestors 'none'",
          "img-src 'self' data: blob:",
          `script-src ${scriptSource}`,
          "style-src 'unsafe-inline'",
        ].join("; "),
        "content-type": "text/html; charset=utf-8",
      },
    },
  );
}

function scriptNonce(): string {
  return crypto.randomUUID().replaceAll("-", "");
}

function shortTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toISOString().replace(".000Z", "Z");
}

function formatDuration(value: number | undefined): string {
  if (!Number.isFinite(value)) {
    return "-";
  }
  const seconds = Math.max(0, Math.round((value ?? 0) / 1000));
  if (seconds < 60) {
    return `${seconds}s`;
  }
  const minutes = Math.floor(seconds / 60);
  const rest = seconds % 60;
  return `${minutes}m ${rest}s`;
}

function formatExitCode(value: number | undefined): string {
  return Number.isFinite(value) ? String(value) : "-";
}

function formatBytes(value: number): string {
  if (value < 1024) {
    return `${value} B`;
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KiB`;
  }
  return `${(value / 1024 / 1024).toFixed(1)} MiB`;
}

function truncate(value: string, maxLength: number): string {
  return value.length > maxLength ? `${value.slice(0, maxLength - 1)}...` : value;
}

function escapeHTML(value: string | undefined): string {
  return (value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}
