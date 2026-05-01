import { issueUserToken, sha256Hex } from "./auth";
import { errorMessage, json, readJson } from "./http";
import type { Env, Provider } from "./types";
import { requestOrg } from "./usage";

const githubAuthorizeURL = "https://github.com/login/oauth/authorize";
const githubTokenURL = "https://github.com/login/oauth/access_token";
const githubAPIURL = "https://api.github.com";

interface OAuthPending {
  id: string;
  state: string;
  pollSecretHash: string;
  provider?: Provider;
  createdAt: string;
  expiresAt: string;
  token?: string;
  owner?: string;
  org?: string;
  login?: string;
  error?: string;
}

interface GitHubUser {
  login?: string;
  name?: string | null;
  email?: string | null;
}

interface GitHubEmail {
  email?: string;
  primary?: boolean;
  verified?: boolean;
}

export async function githubAuthRoute(
  request: Request,
  action: string | undefined,
  storage: DurableObjectStorage,
  env: Env,
): Promise<Response> {
  const method = request.method.toUpperCase();
  if (method === "POST" && action === "start") {
    return await githubAuthStart(request, storage, env);
  }
  if (method === "GET" && action === "callback") {
    return await githubAuthCallback(request, storage, env);
  }
  if (method === "POST" && action === "poll") {
    return await githubAuthPoll(request, storage);
  }
  return json({ error: "not_found" }, { status: 404 });
}

async function githubAuthStart(
  request: Request,
  storage: DurableObjectStorage,
  env: Env,
): Promise<Response> {
  const clientID = env.CRABBOX_GITHUB_CLIENT_ID;
  if (!clientID || !env.CRABBOX_GITHUB_CLIENT_SECRET) {
    return json(
      { error: "github_oauth_not_configured", message: "GitHub OAuth is not configured" },
      { status: 503 },
    );
  }
  const input = await readJson<{
    pollSecretHash?: string;
    provider?: Provider;
  }>(request);
  if (!input.pollSecretHash || !/^[a-f0-9]{64}$/.test(input.pollSecretHash)) {
    return json({ error: "invalid_poll_secret_hash" }, { status: 400 });
  }
  const id = randomID("login");
  const state = randomID("state");
  const now = new Date();
  const expiresAt = new Date(now.getTime() + 10 * 60 * 1000);
  const pending: OAuthPending = {
    id,
    state,
    pollSecretHash: input.pollSecretHash,
    createdAt: now.toISOString(),
    expiresAt: expiresAt.toISOString(),
  };
  if (input.provider === "aws" || input.provider === "hetzner") {
    pending.provider = input.provider;
  }
  await storage.put(oauthKey(id), pending);
  await storage.put(oauthStateKey(state), id);

  const authorize = new URL(githubAuthorizeURL);
  authorize.searchParams.set("client_id", clientID);
  authorize.searchParams.set("redirect_uri", githubRedirectURI(request, env));
  authorize.searchParams.set("scope", "read:user user:email");
  authorize.searchParams.set("state", state);
  return json({
    loginID: id,
    url: authorize.toString(),
    expiresAt: expiresAt.toISOString(),
  });
}

async function githubAuthCallback(
  request: Request,
  storage: DurableObjectStorage,
  env: Env,
): Promise<Response> {
  const url = new URL(request.url);
  const code = url.searchParams.get("code") ?? "";
  const state = url.searchParams.get("state") ?? "";
  const error = url.searchParams.get("error") ?? "";
  const id = state ? await storage.get<string>(oauthStateKey(state)) : undefined;
  const pending = id ? await storage.get<OAuthPending>(oauthKey(id)) : undefined;
  if (!pending || pending.state !== state || Date.parse(pending.expiresAt) <= Date.now()) {
    return html(
      "Crabbox login expired",
      "The login request expired. Run crabbox login again.",
      400,
    );
  }
  if (error || !code) {
    pending.error = error || "missing_code";
    await storage.put(oauthKey(pending.id), pending);
    return html("Crabbox login failed", "GitHub did not authorize the login.", 400);
  }
  try {
    const accessToken = await exchangeGitHubCode(code, githubRedirectURI(request, env), env);
    const identity = await githubIdentity(accessToken);
    const org = requestOrg(
      new Request(request.url, {
        headers: { "x-crabbox-org": env.CRABBOX_DEFAULT_ORG ?? "openclaw" },
      }),
      env,
    );
    const tokenInput = {
      owner: identity.owner,
      org,
      login: identity.login,
    } as { owner: string; org: string; login: string; name?: string };
    if (identity.name) {
      tokenInput.name = identity.name;
    }
    pending.token = await issueUserToken(env, tokenInput);
    pending.owner = identity.owner;
    pending.org = org;
    pending.login = identity.login;
    await storage.put(oauthKey(pending.id), pending);
    return html(
      "Crabbox login complete",
      "GitHub authorized Crabbox. You can close this tab and return to the terminal.",
    );
  } catch (err) {
    pending.error = errorMessage(err);
    await storage.put(oauthKey(pending.id), pending);
    return html("Crabbox login failed", "The coordinator could not finish GitHub login.", 500);
  }
}

async function githubAuthPoll(request: Request, storage: DurableObjectStorage): Promise<Response> {
  const input = await readJson<{ loginID?: string; pollSecret?: string }>(request);
  if (!input.loginID || !input.pollSecret) {
    return json({ error: "invalid_poll" }, { status: 400 });
  }
  const pending = await storage.get<OAuthPending>(oauthKey(input.loginID));
  if (!pending || Date.parse(pending.expiresAt) <= Date.now()) {
    return json({ status: "expired" }, { status: 410 });
  }
  if ((await sha256Hex(input.pollSecret)) !== pending.pollSecretHash) {
    return json({ error: "forbidden" }, { status: 403 });
  }
  if (pending.error) {
    await deletePendingOAuth(storage, pending);
    return json({ status: "failed", error: pending.error }, { status: 400 });
  }
  if (!pending.token) {
    return json({ status: "pending", expiresAt: pending.expiresAt });
  }
  const response = {
    status: "complete",
    token: pending.token,
    owner: pending.owner,
    org: pending.org,
    login: pending.login,
    provider: pending.provider,
  };
  await deletePendingOAuth(storage, pending);
  return json(response);
}

function githubRedirectURI(request: Request, env: Env): string {
  const base = env.CRABBOX_PUBLIC_URL || new URL(request.url).origin;
  return `${base.replace(/\/+$/, "")}/v1/auth/github/callback`;
}

async function exchangeGitHubCode(code: string, redirectURI: string, env: Env): Promise<string> {
  const body = new URLSearchParams({
    client_id: env.CRABBOX_GITHUB_CLIENT_ID ?? "",
    client_secret: env.CRABBOX_GITHUB_CLIENT_SECRET ?? "",
    code,
    redirect_uri: redirectURI,
  });
  const response = await fetch(githubTokenURL, {
    method: "POST",
    headers: {
      accept: "application/json",
      "content-type": "application/x-www-form-urlencoded",
      "user-agent": "crabbox-coordinator",
    },
    body,
  });
  const data = (await response.json()) as { access_token?: string; error?: string };
  if (!response.ok || !data.access_token) {
    throw new Error(data.error || `github token exchange failed: ${response.status}`);
  }
  return data.access_token;
}

async function githubIdentity(accessToken: string): Promise<{
  owner: string;
  login: string;
  name?: string;
}> {
  const headers = {
    accept: "application/vnd.github+json",
    authorization: `Bearer ${accessToken}`,
    "user-agent": "crabbox-coordinator",
    "x-github-api-version": "2022-11-28",
  };
  const userResponse = await fetch(`${githubAPIURL}/user`, { headers });
  if (!userResponse.ok) {
    throw new Error(`github user lookup failed: ${userResponse.status}`);
  }
  const user = (await userResponse.json()) as GitHubUser;
  const login = user.login || "unknown";
  let owner = user.email || "";
  const emailResponse = await fetch(`${githubAPIURL}/user/emails`, { headers });
  if (emailResponse.ok) {
    const emails = (await emailResponse.json()) as GitHubEmail[];
    owner =
      emails.find((email) => email.primary && email.verified)?.email ||
      emails.find((email) => email.verified)?.email ||
      owner;
  }
  if (!owner) {
    owner = `${login}@users.noreply.github.com`;
  }
  const identity = {
    owner,
    login,
  } as { owner: string; login: string; name?: string };
  if (user.name) {
    identity.name = user.name;
  }
  return identity;
}

async function deletePendingOAuth(
  storage: DurableObjectStorage,
  pending: OAuthPending,
): Promise<void> {
  await storage.delete(oauthKey(pending.id));
  await storage.delete(oauthStateKey(pending.state));
}

function oauthKey(id: string): string {
  return `oauth:${id}`;
}

function oauthStateKey(state: string): string {
  return `oauth_state:${state}`;
}

function randomID(prefix: string): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return `${prefix}_${[...bytes].map((byte) => byte.toString(16).padStart(2, "0")).join("")}`;
}

function html(title: string, message: string, status = 200): Response {
  const escapedTitle = escapeHTML(title);
  const escapedMessage = escapeHTML(message);
  return new Response(
    `<!doctype html><html><head><meta charset="utf-8"><title>${escapedTitle}</title><style>body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;max-width:42rem;margin:5rem auto;padding:0 1rem;line-height:1.5;color:#111}code{background:#f4f4f5;padding:.15rem .3rem;border-radius:4px}</style></head><body><h1>${escapedTitle}</h1><p>${escapedMessage}</p></body></html>`,
    { status, headers: { "content-type": "text/html; charset=utf-8" } },
  );
}

function escapeHTML(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}
