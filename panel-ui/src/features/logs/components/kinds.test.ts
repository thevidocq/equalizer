import { describe, expect, it } from "vitest";
import { kindOf, reasonLabel, explanation, defaultKinds } from "./kinds";

describe("log kinds", () => {
  it("groups types and words reasons", () => {
    expect(kindOf("exposure.alert").id).toBe("alert");
    expect(reasonLabel({ ts: "", type: "request.blocked", reason: "authz_not_owned" })).toBe("Object of another user");
    expect(reasonLabel({ ts: "", type: "field.rejected", reason: "rule_violation", field: "q" })).toBe("q: broke a rule");
    expect(reasonLabel({ ts: "", type: "request.blocked", reason: "brand_new_code" })).toBe("brand new code");
    expect(explanation({ ts: "", type: "ratelimit.hit" })).toMatch(/Too many requests/);
  });
  it("opens on what was refused", () => {
    expect(defaultKinds).not.toContain("served");
  });
});
