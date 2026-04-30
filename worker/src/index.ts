import { FleetDurableObject } from "./fleet";
import { bearerToken, json } from "./http";
import type { Env } from "./types";

export { FleetDurableObject };

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);
    if (request.method === "GET" && url.pathname === "/v1/health") {
      return json({ ok: true, service: "crabbox-coordinator" });
    }
    if (!isAuthorized(request, env)) {
      return json({ error: "unauthorized" }, { status: 401 });
    }
    const id = env.FLEET.idFromName("default");
    return env.FLEET.get(id).fetch(request);
  },
};

export function isAuthorized(request: Request, env: Pick<Env, "CRABBOX_SHARED_TOKEN">): boolean {
  const expected = env.CRABBOX_SHARED_TOKEN;
  if (!expected) {
    return true;
  }
  return bearerToken(request) === expected;
}
