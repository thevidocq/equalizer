import { describe, expect, it } from "vitest";
import { bySeverity, closedByMap, shortURL, stillOpen, verdictOf } from "./verdicts";
import type { SecurityFinding } from "../hooks/useSecurity";

const f = (severity: SecurityFinding["severity"], verdict: SecurityFinding["map"]["verdict"]): SecurityFinding =>
  ({ id: severity + verdict, source: "nikto", title: "t", severity, cve: [], cwe: [], map: { verdict } });

describe("findings", () => {
  it("counts by severity, the most severe first", () => {
    expect(bySeverity([f("low", "server"), f("high", "area"), f("high", "refused")])).toEqual([
      { severity: "high", count: 2 }, { severity: "low", count: 1 }]);
  });
  it("counts what Equalizer leaves open", () => {
    expect(stillOpen([f("high", "area"), f("high", "free_text"), f("high", "refused"), f("low", "precise"), f("low", "server")])).toBe(2);
  });
  it("counts what the map closes", () => {
    expect(closedByMap([f("high", "refused"), f("high", "precise"), f("high", "area"), f("low", "server")])).toBe(2);
  });
  it("never calls closed what the scanner reached through Equalizer", () => {
    expect(verdictOf(f("high", "refused"), "protected").label).toBe("Closed now");
    expect(verdictOf(f("high", "refused"), "original").label).toBe("Closed");
  });
  it("shows the path of an address", () => {
    expect(shortURL("https://www.example.com/a/b?q=1")).toBe("/a/b?q=1");
    expect(shortURL("not a url")).toBe("not a url");
    expect(shortURL(null)).toBe("");
  });
});
