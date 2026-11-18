import { describe, expect, it } from "vitest";
import { parseMap, STARTER, toEditorYaml } from "./yaml";

describe("map YAML", () => {
  it("parses the starter map", () => {
    const p = parseMap(STARTER);
    expect(p.ok).toBe(true);
    if (p.ok) expect((p.value.routes as unknown[]).length).toBe(1);
  });
  it("reports the line of a syntax error", () => {
    const p = parseMap("roles:\n  list: [a\n");
    expect(p.ok).toBe(false);
    if (!p.ok) expect(p.message).toMatch(/^Line \d+/);
  });
  it("rejects a list at the top level", () => {
    expect(parseMap("- a\n- b\n").ok).toBe(false);
  });
  it("keeps meta out of the editor", () => {
    const text = toEditorYaml({ meta: { version: 3 }, roles: { list: { guest: {} } } });
    expect(text).not.toContain("meta");
    expect(text).toContain("guest");
  });
  it("does not turn 'on' into a boolean", () => {
    const p = parseMap("on: x\n");
    expect(p.ok && "on" in p.value).toBe(true);
  });
});

describe("editor order", () => {
  it("puts keys in the order of the format", () => {
    const text = toEditorYaml({ routes: [{ risk: "low", id: "home", path: "/" }], legacy: { reach: {}, base_url: "x" } });
    expect(text.indexOf("legacy:")).toBeLessThan(text.indexOf("routes:"));
    expect(text.indexOf("id: home")).toBeLessThan(text.indexOf("risk: low"));
    expect(text.indexOf("base_url")).toBeLessThan(text.indexOf("reach"));
  });
});
