import { describe, expect, it } from "vitest";
import { identicalHint } from "./VerificationPanel";

describe("identicalHint", () => {
  it("says nothing more when no page changes by itself", () => {
    expect(identicalHint(0)).toBe("The site works as before.");
  });
  it("counts the pages the site changes by itself", () => {
    expect(identicalHint(1)).toContain("1 page changes by itself");
    expect(identicalHint(3)).toContain("3 pages change by themselves");
  });
});
