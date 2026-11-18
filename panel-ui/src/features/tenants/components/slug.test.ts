import { describe, expect, it } from "vitest";
import { slugify } from "./slug";

describe("slugify", () => {
  it("makes ids from names", () => {
    expect(slugify("Città di Esempio")).toBe("citta-di-esempio");
    expect(slugify("  ACME — Portal 2 ")).toBe("acme-portal-2");
    expect(slugify("!!!")).toBe("");
  });
});
