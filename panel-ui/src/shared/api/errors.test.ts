import { describe, expect, it } from "vitest";
import { ApiError } from "./client";
import { describeError } from "./errors";

describe("describeError", () => {
  it("never says nothing", () => {
    expect(describeError(new ApiError(424, "ai_failed", ""))).toMatch(/Settings → AI/);
    expect(describeError(new ApiError(500, "unknown", ""))).toBe("The panel could not complete the request (500).");
    expect(describeError(new ApiError(424, "ai_failed", "no credit left on the AI account"))).toBe("no credit left on the AI account");
    expect(describeError(new Error(""))).toBe("The panel could not complete the request.");
  });
});
