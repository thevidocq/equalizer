import { describe, expect, it } from "vitest";
import { describeEntry } from "./describe";

describe("describeEntry", () => {
  it("reads like a sentence", () => {
    expect(describeEntry({ id: 1, ts: "", action: "map.approve", target: "portal", user: "ap@acme.test", detail: { version: 3 } }))
      .toBe("ap@acme.test approved version 3 of portal");
    expect(describeEntry({ id: 2, ts: "", action: "auth.login_failed", target: "", user: null }))
      .toBe("Someone failed to log in");
    expect(describeEntry({ id: 3, ts: "", action: "user.invite", target: "ed@acme.test", user: "boss@acme.test" }))
      .toBe("boss@acme.test invited ed@acme.test");
  });
});
