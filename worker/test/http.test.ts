import { describe, expect, it } from "vitest";

import { isAuthorized } from "../src";

describe("coordinator auth", () => {
  it("allows requests when no shared token is configured", () => {
    const request = new Request("https://example.test/v1/pool");
    expect(isAuthorized(request, {})).toBe(true);
  });

  it("requires the configured bearer token", () => {
    const denied = new Request("https://example.test/v1/pool");
    const allowed = new Request("https://example.test/v1/pool", {
      headers: { authorization: "Bearer secret" },
    });
    expect(isAuthorized(denied, { CRABBOX_SHARED_TOKEN: "secret" })).toBe(false);
    expect(isAuthorized(allowed, { CRABBOX_SHARED_TOKEN: "secret" })).toBe(true);
  });
});
