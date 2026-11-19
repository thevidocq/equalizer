import { describe, expect, it } from "vitest";
import { applyAuth, problems, readAuth } from "./auth";

const doc = { meta: { app: "a" }, roles: { anonymous: "guest", list: { guest: {}, staff: {} } }, routes: [] };

describe("login settings", () => {
  it("is off by default", () => {
    expect(readAuth(doc).on).toBe(false);
    expect(applyAuth(doc, readAuth(doc)).roles).toEqual(doc.roles);
  });
  it("writes roles.auth and drops meta", () => {
    const f = { ...readAuth(doc), on: true, issuer: "https://login.example/v2.0", client_id: "abc",
      roles: [{ group: "staff-group", role: "staff" }, { group: " ", role: "guest" }] };
    expect(problems(f, doc)).toEqual([]);
    const out = applyAuth(doc, f) as { meta?: unknown; roles: { auth: { roles: unknown[]; scopes: string[] } } };
    expect(out.meta).toBeUndefined();
    expect(out.roles.auth.roles).toEqual([{ group: "staff-group", role: "staff" }]);
    expect(out.roles.auth.scopes).toEqual(["openid", "email", "profile"]);
    expect(readAuth(out as never).on).toBe(true);
  });
  it("refuses an http issuer", () => {
    expect(problems({ ...readAuth(doc), on: true, issuer: "http://x", client_id: "a", roles: [{ group: "g", role: "staff" }] }, doc).length).toBe(1);
  });
});
