/** roles.auth of the map, as the Login tab edits it. */
export type RoleRule = { group: string; role: string };
export type AuthForm = {
  on: boolean;
  issuer: string;
  client_id: string;
  scopes: string;
  groups_claim: string;
  /** site: every page; paths: only the listed paths; optional: login at /.eq/oidc/login */
  scope: "site" | "paths" | "optional";
  paths: string;
  roles: RoleRule[];
};

const pathRe = /^\/[A-Za-z0-9._~%!$&'()*+,=:@/-]{0,200}$/;

export function pathList(text: string): string[] {
  return text.split(/[\s,]+/).map((p) => p.trim()).filter(Boolean);
}

type Doc = Record<string, unknown>;
type Roles = { anonymous?: string; list?: Record<string, unknown>; auth?: Record<string, unknown> };

export function rolesOf(doc: Doc | undefined): string[] {
  return Object.keys(((doc?.roles as Roles | undefined)?.list) ?? {});
}

/** Reads the form from a map: off when the map says nothing (the default). */
export function readAuth(doc: Doc | undefined): AuthForm {
  const a = ((doc?.roles as Roles | undefined)?.auth ?? {}) as Record<string, unknown>;
  const on = a.provider === "oidc";
  return {
    on,
    issuer: on ? String(a.issuer ?? "") : "",
    client_id: on ? String(a.client_id ?? "") : "",
    scopes: Array.isArray(a.scopes) ? a.scopes.join(" ") : "openid email profile",
    groups_claim: String(a.groups_claim ?? "groups"),
    scope: Array.isArray(a.paths) && a.paths.length ? "paths" : a.required === false ? "optional" : "site",
    paths: Array.isArray(a.paths) ? a.paths.join("\n") : "",
    roles: Array.isArray(a.roles) && a.roles.length ? (a.roles as RoleRule[]) : [{ group: "", role: rolesOf(doc)[0] ?? "" }],
  };
}

/** Problems that would make the draft fail; empty when the form is fine. */
export function problems(f: AuthForm, doc: Doc | undefined): string[] {
  if (!f.on) return [];
  const out: string[] = [];
  if (!/^https:\/\/\S+$/.test(f.issuer)) out.push("The issuer must be an https address.");
  if (!f.client_id.trim()) out.push("The client ID is required.");
  const rules = f.roles.filter((r) => r.group.trim());
  if (!rules.length) out.push("Add at least one group and its role.");
  if (f.scope !== "site" && !(doc?.roles as Roles | undefined)?.anonymous) out.push("Login on some paths or optional needs an anonymous role in the map.");
  if (f.scope === "paths") {
    const list = pathList(f.paths);
    if (!list.length) out.push("List at least one path, e.g. /wp-admin/.");
    if (list.length > 50) out.push("At most 50 paths.");
    const bad = list.filter((p) => !pathRe.test(p));
    if (bad.length) out.push(`Paths start with / and have no spaces: ${bad.join(", ")}.`);
  }
  return out;
}

/** The map with roles.auth set from the form (or removed when off). */
export function applyAuth(doc: Doc, f: AuthForm): Doc {
  const copy = JSON.parse(JSON.stringify(doc)) as Doc;
  const { meta: _meta, ...rest } = copy;
  const roles = { ...(rest.roles as Roles) };
  if (!f.on) {
    delete roles.auth;
  } else {
    roles.auth = {
      provider: "oidc",
      issuer: f.issuer.trim(),
      client_id: f.client_id.trim(),
      scopes: f.scopes.split(/\s+/).filter(Boolean),
      groups_claim: f.groups_claim.trim() || "groups",
      roles: f.roles.filter((r) => r.group.trim()).map((r) => ({ group: r.group.trim(), role: r.role })),
      ...(f.scope === "paths" ? { paths: pathList(f.paths) } : { required: f.scope === "site" }),
    };
  }
  return { ...rest, roles };
}
