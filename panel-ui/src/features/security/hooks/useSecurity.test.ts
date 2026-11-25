import { describe, expect, it } from "vitest";
import { going, pollEvery, type SecurityOverview } from "./useSecurity";

const now = Date.parse("2026-11-24T15:00:00Z");
const scan = (status: "pending" | "running" | "completed" | "failed", extra = {}) => ({
  id: 1, target: "protected" as const, domain: "d", scan_type: "quick" as const, status, created_at: "2026-11-24T14:59:00Z", ...extra,
});
const ov = (s: ReturnType<typeof scan>): SecurityOverview =>
  ({ configured: true, targets: [{ target: "protected", available: true, findings: [], scan: s }] });

describe("going", () => {
  it("polls while a scan goes on", () => {
    expect(going(ov(scan("running")), now)).toBe(true);
  });
  it("polls a few minutes for the draft of a scan just ended", () => {
    expect(going(ov(scan("completed", { completed_at: "2026-11-24T14:59:30Z" })), now)).toBe(true);
    expect(going(ov(scan("completed", { completed_at: "2026-11-24T14:59:30Z", draft_note: "nothing" })), now)).toBe(false);
    expect(going(ov(scan("completed", { completed_at: "2026-11-24T14:00:00Z" })), now)).toBe(false);
  });
  it("keeps reading, slowly, while the draft waits for the map, up to a day", () => {
    expect(pollEvery(ov(scan("completed", { completed_at: "2026-11-24T14:00:00Z" })), now)).toBe(60_000);
    expect(pollEvery(ov(scan("completed", { completed_at: "2026-11-23T14:00:00Z" })), now)).toBe(false);
    expect(pollEvery(ov(scan("completed", { completed_at: "2026-11-24T14:00:00Z", draft_version: 4 })), now)).toBe(false);
    expect(pollEvery(ov(scan("failed", { completed_at: "2026-11-24T14:00:00Z" })), now)).toBe(false);
  });
});
