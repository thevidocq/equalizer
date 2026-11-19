import yaml from "js-yaml";

export type Parsed = { ok: true; value: Record<string, unknown> } | { ok: false; message: string };

/** Parses the editor's YAML into the JSON document sent to the panel. */
export function parseMap(text: string): Parsed {
  try {
    const value = yaml.load(text, { schema: yaml.JSON_SCHEMA });
    if (!value || typeof value !== "object" || Array.isArray(value)) {
      return { ok: false, message: "The map must be a YAML mapping (keys and values), not a list or a single value." };
    }
    return { ok: true, value: value as Record<string, unknown> };
  } catch (e) {
    const err = e as { mark?: { line: number }; reason?: string };
    if (err.mark) return { ok: false, message: `Line ${err.mark.line + 1}: ${err.reason ?? "invalid YAML"}` };
    return { ok: false, message: "Invalid YAML." };
  }
}

// The database does not keep key order: show keys in the order of the format.
const ORDER = [
  "legacy", "roles", "defaults", "types", "routes", "forms", "flows", "api", "compat", "callbacks", "assets",
  "external_links", "behavior",
  "id", "path", "route", "prefix", "method", "roles", "params", "query", "fields", "legacy_request",
  "render", "links", "forms", "success", "session", "on_success", "rate_limit",
  "base_url", "reach", "mode", "connector_id", "allow", "anonymous", "auth", "list",
  "type", "required", "min", "max", "values", "pattern", "charset", "max_length", "rules",
  "origin", "confidence", "risk", "approved_by",
];
const rank = (k: string) => {
  const i = ORDER.indexOf(k);
  return i < 0 ? ORDER.length : i;
};

function ordered(v: unknown): unknown {
  if (Array.isArray(v)) return v.map(ordered);
  if (!v || typeof v !== "object") return v;
  const entries = Object.entries(v as Record<string, unknown>).sort(([a], [b]) => rank(a) - rank(b));
  return Object.fromEntries(entries.map(([k, x]) => [k, ordered(x)]));
}

/** The document as YAML for the editor, without the meta the panel owns. */
export function toEditorYaml(doc: Record<string, unknown> | undefined): string {
  if (!doc) return STARTER;
  const { meta: _meta, ...rest } = doc;
  return yaml.dump(ordered(rest), { lineWidth: 120, noRefs: true });
}

export const STARTER = `# The panel fills in meta (tenant, site, version, status).
legacy:
  base_url: http://10.20.0.15:8080
  reach:
    mode: connector
    connector_id: cn-site-01
    allow: ["10.20.0.15:8080"]

roles:
  anonymous: guest
  list:
    guest: {}

# The whole site, admin included, served as the legacy site serves it.
areas:
  - id: site
    prefix: /
    methods: [GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS]
    max_body_kb: 51200
    origin: human
    risk: medium

# Precise entries add typed checks where they match.
routes:
  - id: home
    path: /
    method: GET
    roles: [guest]
    legacy_request: { method: GET, path: /index.jsp }
    render: faithful
    origin: human
    risk: low
`;
