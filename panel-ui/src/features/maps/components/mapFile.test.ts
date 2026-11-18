import { describe, expect, it } from "vitest";
import { exportName, readMapFile } from "./mapFile";

const file = (text: string, name = "m.yaml") => new File([text], name);

describe("map files", () => {
  it("reads YAML and JSON, drops meta", async () => {
    const y = await readMapFile(file("meta: { tenant: other }\nlegacy: { base_url: http://x }\nroles: { anonymous: guest }\n"));
    expect(y.ok && "meta" in y.value).toBe(false);
    expect(y.ok && y.value.legacy).toEqual({ base_url: "http://x" });
    const j = await readMapFile(file('{"legacy":{},"roles":{}}', "m.json"));
    expect(j.ok).toBe(true);
  });
  it("refuses what is not a map", async () => {
    expect((await readMapFile(file("- a\n- b\n"))).ok).toBe(false);
    expect((await readMapFile(file("routes: []\n"))).ok).toBe(false);
    expect((await readMapFile(file("x".repeat(2 * 1024 * 1024 + 1)))).ok).toBe(false);
  });
  it("names the export", () => expect(exportName("portal", 3)).toBe("portal-map-v3.yaml"));
});
