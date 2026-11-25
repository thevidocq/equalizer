import { describe, expect, it } from "vitest";
import { surfaceLine } from "./securityStudy";

describe("surfaceLine", () => {
  it("counts what an attacker can reach", () => {
    expect(surfaceLine({ pages: 12, forms: 3, uploads: 1, free_text_fields: 7, code_entries: 4, code_entries_public: 2, risky_calls: 9 }))
      .toBe("12 pages · 3 forms · 1 file upload · 7 free-text fields · 4 entry points only in the code (2 without login) · 9 risky calls in the code");
  });
  it("leaves out what the site does not have", () => {
    expect(surfaceLine({ pages: 2, forms: 0, free_text_fields: 0 })).toBe("2 pages · 0 forms · 0 free-text fields");
    expect(surfaceLine(undefined)).toBe("");
  });
});
