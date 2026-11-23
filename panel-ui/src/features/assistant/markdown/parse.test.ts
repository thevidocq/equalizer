import { describe, expect, it } from "vitest";
import { inline, parse } from "./parse";

describe("parse", () => {
  it("reads paragraphs, lists, tables and code", () => {
    const b = parse("It was **a scan**.\n\n- one\n- `two`\n\n1. first\n2. second\n\n| Reason | Visitors |\n| --- | --- |\n| not in map | 11 |\n\n```\nGET /x\n```");
    expect(b.map((x) => x.t)).toEqual(["p", "ul", "ol", "table", "code"]);
    const table = b[3];
    expect(table?.t === "table" && table.rows[0]?.[1]).toEqual([{ t: "text", v: "11" }]);
  });
  it("keeps links only toward the panel", () => {
    expect(inline("[Logs](/t/acme/logs?q=9c41)")).toEqual([{ t: "link", v: "Logs", to: "/t/acme/logs?q=9c41" }]);
    expect(inline("[click](https://evil.test)")).toEqual([{ t: "text", v: "click" }]);
    expect(inline("[x](javascript:alert(1))")[0]).toMatchObject({ t: "text" });
  });
  it("ends on any input", () => {
    for (const s of ["## Title\rmore", "# a\u2028b", "ok\n# a\rb", "```\nopen", "| a |\n| --- |", "-", "1.", "#"]) {
      expect(Array.isArray(parse(s))).toBe(true);
    }
    expect(inline("[x](/t/acme/../../login)")).toEqual([{ t: "text", v: "x" }]);
  });
  it("leaves markup as text", () => {
    expect(inline("<img src=x onerror=alert(1)>")).toEqual([{ t: "text", v: "<img src=x onerror=alert(1)>" }]);
  });
});
