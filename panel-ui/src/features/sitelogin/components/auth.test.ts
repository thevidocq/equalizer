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

describe("where the login is asked", () => {
  const on = { ...readAuth(doc), on: true, issuer: "https://login.example/v2.0", client_id: "abc", roles: [{ group: "*", role: "staff" }] };
  it("writes the paths instead of required", () => {
    const f = { ...on, scope: "paths" as const, paths: "/wp-admin/\n/wp-login.php, /xmlrpc.php" };
    expect(problems(f, doc)).toEqual([]);
    const auth = (applyAuth(doc, f) as { roles: { auth: Record<string, unknown> } }).roles.auth;
    expect(auth.paths).toEqual(["/wp-admin/", "/wp-login.php", "/xmlrpc.php"]);
    expect(auth.required).toBeUndefined();
    expect(readAuth(applyAuth(doc, f) as never).scope).toBe("paths");
  });
  it("checks the paths", () => {
    expect(problems({ ...on, scope: "paths", paths: "" }, doc)).toHaveLength(1);
    expect(problems({ ...on, scope: "paths", paths: "wp-admin" }, doc)).toHaveLength(1);
    expect(problems({ ...on, scope: "paths", paths: "/a" }, { ...doc, roles: { list: { staff: {} } } })).toHaveLength(1);
  });
  it("keeps the whole site and optional", () => {
    expect((applyAuth(doc, { ...on, scope: "site" }) as { roles: { auth: { required: boolean } } }).roles.auth.required).toBe(true);
    expect(readAuth(applyAuth(doc, { ...on, scope: "optional" }) as never).scope).toBe("optional");
  });
});
