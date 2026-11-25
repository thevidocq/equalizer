import { describe, expect, it } from "vitest";
import { discoveryPoll, type Discovery } from "./useDiscovery";

const d = (status: Discovery["status"], report: unknown = null) => ({ id: 1, app: "blog", status, created_at: "", report }) as Discovery;

describe("discoveryPoll", () => {
  it("reads a running discovery often, a pending study slowly, a finished one never", () => {
    expect(discoveryPoll(d("running"))).toBe(4000);
    expect(discoveryPoll(d("done", { security: { used: false, pending: true } }))).toBe(15000);
    expect(discoveryPoll(d("done", { security: { used: true } }))).toBe(false);
    expect(discoveryPoll(d("failed", { security: { pending: true } }))).toBe(false);
    expect(discoveryPoll(undefined)).toBe(false);
  });
});
