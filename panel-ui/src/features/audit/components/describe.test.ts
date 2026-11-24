import { describe, expect, it } from "vitest";
import { describeEntry, isWarning } from "./describe";
import { toQuery } from "./AuditFilters";

describe("describeEntry", () => {
  it("reads like a sentence", () => {
    expect(describeEntry({ id: 1, ts: "", action: "map.publish", target: "portal", user: "ap@acme.test", detail: { version: 3 } }))
      .toBe("ap@acme.test published version 3 of the map of portal");
    expect(describeEntry({ id: 2, ts: "", action: "auth.login_failed", target: "", user: null }))
      .toBe("Someone failed to log in");
    expect(describeEntry({ id: 3, ts: "", action: "user.invite", target: "ed@acme.test", user: "boss@acme.test" }))
      .toBe("boss@acme.test invited ed@acme.test");
  });
  it("tells security scans, and who confirmed the original site", () => {
    expect(describeEntry({ id: 30, ts: "", action: "security.scan", target: "shop", user: "a@acme.test",
      detail: { target: "original", domain: "www.shop.example", scan_type: "standard" } }))
      .toBe("a@acme.test started a standard scan of the original site of shop (www.shop.example), confirming it is theirs or allowed in writing");
    const failed = { id: 31, ts: "", action: "security.scan_failed", target: "shop", user: null, detail: { reason: "timeout" } };
    expect(describeEntry(failed)).toBe("Equalizer could not finish the security scan of shop: timeout");
    expect(isWarning(failed)).toBe(true);
  });
  it("tells the assistant's questions and drafts, never their text", () => {
    expect(describeEntry({ id: 20, ts: "", action: "assistant.asked", target: "shop", user: "a@acme.test", detail: { tools: ["search_logs"], proposals: 1 } }))
      .toBe("a@acme.test asked the assistant about shop (1 change proposed)");
    expect(describeEntry({ id: 21, ts: "", action: "assistant.draft", target: "shop", user: "a@acme.test", detail: { version: 6, from: 5 } }))
      .toBe("a@acme.test made a change proposed by the assistant into version 6 of the map of shop");
    expect(describeEntry({ id: 22, ts: "", action: "assistant.settings", target: "acme", user: "a@acme.test", detail: { keep_days: 7 } }))
      .toBe("a@acme.test set the assistant to keep conversations for 7 days");
  });
  it("names the agent and Equalizer, and never shows raw action names", () => {
    expect(describeEntry({ id: 4, ts: "", action: "discovery.finish", target: "shop", user: null, detail: { draft_version: 5 } }))
      .toBe("The agent finished the discovery of shop: draft version 5 is ready to review");
    expect(describeEntry({ id: 5, ts: "", action: "app.never_public", target: "shop", user: "a@acme.test", detail: { never_public: true } }))
      .toBe("a@acme.test marked shop as never reachable from the internet");
    const sent = { id: 6, ts: "", action: "alert.sent", target: "", user: null,
      detail: { alerts: [{ site: "Shop", check: "Connector", down: true }], to: ["a@acme.test"] } };
    expect(describeEntry(sent)).toBe("Equalizer emailed a@acme.test: Connector failing on Shop");
    expect(isWarning(sent)).toBe(true);
  });
});

describe("toQuery", () => {
  it("turns days into a range in the person's time zone", () => {
    expect(toQuery({ q: "x", range: "days", fromDay: "2026-11-01", toDay: "2026-11-02" }))
      .toEqual({ q: "x", from: "2026-11-01T00:00:00.000Z", to: "2026-11-03T00:00:00.000Z" });
    expect(toQuery({ q: "", range: "24h", fromDay: "", toDay: "" }, Date.parse("2026-11-21T10:00:00Z")).from)
      .toBe("2026-11-20T10:00:00.000Z");
  });
});
