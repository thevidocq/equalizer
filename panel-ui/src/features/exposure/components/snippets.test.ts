import { describe, expect, it } from "vitest";
import { mapSnippet, serverSnippet } from "./snippets";

const input = { domain: "portal.example.org", site: "Portal", internal: "127.0.0.1:8443", connectorIp: "10.20.0.9" };

describe("snippets", () => {
  it("moves the site to an internal listener", () => {
    expect(serverSnippet("apache", "internal", input)).toContain("<VirtualHost 127.0.0.1:8443>");
    expect(serverSnippet("nginx", "internal", input)).toContain("listen 127.0.0.1:8443 ssl;");
    expect(serverSnippet("iis", "internal", input)).toContain("-IPAddress 127.0.0.1 -Port 8443");
  });
  it("lets only the connector in", () => {
    expect(serverSnippet("apache", "restrict", input)).toContain("Require ip 10.20.0.9");
    expect(serverSnippet("nginx", "restrict", input)).toContain("allow 10.20.0.9;\n    deny all;");
  });
  it("writes the map's reach for the chosen way", () => {
    expect(mapSnippet("internal", input, "cn-srv1", "")).toContain('address: "127.0.0.1:8443"');
    expect(mapSnippet("restrict", input, "cn-srv1", "10.20.0.15:443")).toContain('allow: ["10.20.0.15:443"]');
  });
});
