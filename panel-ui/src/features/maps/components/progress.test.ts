import { describe, expect, it } from "vitest";
import { progressText } from "./progress";

describe("progressText", () => {
  it("says where the check is", () => {
    expect(progressText({ step: "pages", done: 119, total: 349 })).toBe("comparing pages with and without Equalizer: page 120 of 349");
    expect(progressText({ step: "attacks", done: 3, total: 31, account: "editor" })).toBe("trying attacks on the gateway: entry 4 of 31 (as editor)");
    expect(progressText(null)).toBe("");
  });
});
