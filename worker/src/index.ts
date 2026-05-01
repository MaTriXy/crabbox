import { authenticateRequest, requestWithAuthContext } from "./auth";
import { FleetDurableObject } from "./fleet";
import { json } from "./http";
import type { Env } from "./types";

export { FleetDurableObject };

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);
    if (request.method === "GET" && url.pathname === "/v1/health") {
      return json({ ok: true, service: "crabbox-coordinator" });
    }
    if (url.pathname.startsWith("/v1/auth/")) {
      const id = env.FLEET.idFromName("default");
      return env.FLEET.get(id).fetch(request);
    }
    const auth = await authenticateRequest(request, env);
    if (!auth?.authorized) {
      return json({ error: "unauthorized" }, { status: 401 });
    }
    const id = env.FLEET.idFromName("default");
    return env.FLEET.get(id).fetch(requestWithAuthContext(request, auth));
  },
};

export async function isAuthorized(
  request: Request,
  env: Pick<Env, "CRABBOX_SHARED_TOKEN" | "CRABBOX_SESSION_SECRET" | "CRABBOX_DEFAULT_ORG">,
): Promise<boolean> {
  return Boolean((await authenticateRequest(request, env))?.authorized);
}
