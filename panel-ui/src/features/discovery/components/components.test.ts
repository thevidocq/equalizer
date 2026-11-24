import { describe, expect, it } from "vitest";
import { mergeComponents } from "./components";

describe("mergeComponents", () => {
  it("lists each component once, with where it was seen", () => {
    expect(mergeComponents(
      [{ name: "theme:astra", version: null }, { name: "library:jquery", version: "3.7.1" }],
      [{ name: "theme:astra", version: "4.1" }, { name: "plugin:akismet", version: "5.6" }],
    )).toEqual([
      { name: "library:jquery", version: "3.7.1", seen: "pages" },
      { name: "plugin:akismet", version: "5.6", seen: "code" },
      { name: "theme:astra", version: "4.1", seen: "pages and code" },
    ]);
  });
});
