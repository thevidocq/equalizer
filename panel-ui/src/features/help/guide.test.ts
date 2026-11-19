import { describe, expect, it } from "vitest";
import { parseGuide, search } from "./guide";
import tenant from "./content/tenant.md?raw";
import admin from "./content/admin.md?raw";

const md = `# Guide

Intro text.

## Getting started

### The panel

Sidebar and sites.

\`\`\`text
## not a heading
\`\`\`

### Roles

Approver and editor.

## Monitoring

Status in short.

### Logs

Export CSV of events.
`;

describe("guide", () => {
  it("parses macros and topics, ignoring code", () => {
    const g = parseGuide(md);
    expect(g.title).toBe("Guide");
    expect(g.intro).toBe("Intro text.");
    expect(g.macros.map((m) => m.title)).toEqual(["Getting started", "Monitoring"]);
    expect(g.macros[0]!.topics.map((t) => t.id)).toEqual(["the-panel", "roles"]);
    expect(g.macros[1]!.topics.map((t) => t.title)).toEqual(["Monitoring", "Logs"]);
    expect(g.topics[0]!.body).toContain("## not a heading");
  });
  it("searches every word, titles first", () => {
    const g = parseGuide(md);
    expect(search(g, "csv").map((h) => h.topic.id)).toEqual(["logs"]);
    expect(search(g, "approver editor")[0]!.topic.id).toBe("roles");
    expect(search(g, "nothing here")).toEqual([]);
  });
  it("reads the real guides", () => {
    for (const text of [tenant, admin]) {
      const g = parseGuide(text);
      expect(g.macros.length).toBeGreaterThan(4);
      expect(new Set(g.topics.map((t) => t.id)).size).toBe(g.topics.length);
      expect(text).not.toContain("../screenshots/");
    }
  });
});
