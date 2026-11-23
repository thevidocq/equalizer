import type { Schemas } from "@/shared/api/client";

export type LogEvent = Schemas["LogEvent"];
export type EventType = LogEvent["type"];
export type Since = "1h" | "24h" | "7d" | "30d";

/** The groups of the filter chips: each covers one or more event types. */
export type Kind = { id: string; label: string; color: string; types: EventType[] };

export const kinds: Kind[] = [
  { id: "blocked", label: "Blocked", color: "#b42318", types: ["request.blocked"] },
  { id: "field", label: "Field refused", color: "#a86a12", types: ["field.rejected"] },
  { id: "rate", label: "Rate limit", color: "#6d3fc0", types: ["ratelimit.hit"] },
  { id: "alert", label: "Alert", color: "#1c2633", types: ["behavior.alert", "exposure.alert", "connector.down", "parse.failed"] },
  { id: "legacy", label: "Legacy error", color: "#6b7684", types: ["legacy.error"] },
  { id: "served", label: "Served", color: "#2b7a4b", types: ["request.served"] },
];

/** Served requests are many: the page opens on what was refused. */
export const defaultKinds = kinds.filter((k) => k.id !== "served").map((k) => k.id);

const alert: Kind = kinds[3] as Kind;

/** Identifies an event across refreshes (its position in the list changes). */
export function eventKey(e: LogEvent): string {
  return `${e.ts}|${e.request_id ?? ""}|${e.type}|${e.path ?? ""}`;
}

export function kindOf(t: EventType): Kind {
  return kinds.find((k) => k.types.includes(t)) ?? alert;
}

const reasons: Record<string, string> = {
  route_not_mapped: "Not in the map",
  unknown_host: "Unknown site name",
  method_not_allowed: "Wrong method",
  authz_denied: "Role not allowed",
  authz_not_owned: "Object of another user",
  authz_never: "Closed to this role",
  cross_origin: "Call from another site",
  cross_site_request: "Form sent from another site",
  form_token_mismatch: "Form token not valid",
  asset_name_rejected: "File name refused",
  behavior_block: "Blocked by a behaviour rule",
  behavior_throttle: "Slowed down",
  legacy_unreachable: "Legacy site unreachable",
  scheme_mismatch: "Site settings disagree on http/https",
  connector_unavailable: "Connector offline",
  response_too_large: "Answer too large",
  file_too_large: "File too large",
  file_type_not_allowed: "File type not allowed",
  file_unreadable: "Image not readable",
  type_mismatch: "Wrong type",
  too_long: "Too long",
  pattern_mismatch: "Wrong format",
  rule_violation: "Broke a rule",
  missing_required: "Missing",
  unknown_field: "Field not in the map",
  value_not_allowed: "Value not allowed",
  out_of_range: "Out of range",
  charset_violation: "Characters not allowed",
  login_required: "Login required",
  oidc_no_role: "No role for the user's groups",
  oidc_invalid_token: "Identity provider token refused",
  oidc_bad_callback: "Login answer not valid",
  oidc_no_pending_login: "Login answer without a login",
  oidc_exchange_failed: "Identity provider did not answer",
  body_too_large: "Upload or body too large",
  malformed_body: "Body not readable",
  unsupported_body: "Kind of body not accepted",
  raw_type: "File not what it claims",
  websocket_not_supported: "WebSocket not supported yet",
};

/** A reason code in words. */
export function reasonText(reason: string): string {
  return reasons[reason] ?? reason.replaceAll("_", " ");
}

/** What happened, for events of the Alert group (they have no reason code). */
const titles: Partial<Record<EventType, string>> = {
  "connector.down": "Connector offline",
  "exposure.alert": "Site reachable without Equalizer",
  "behavior.alert": "Visitor behaving like a scanner",
  "parse.failed": "Page could not be rebuilt",
};

/** A short line for the table: the field name first when there is one. */
export function reasonLabel(e: LogEvent): string {
  return subtitle(e) ?? kindOf(e.type).label;
}

/** The line under the kind, or null when it would only repeat the kind ("Served"). */
export function subtitle(e: LogEvent): string | null {
  if (e.reason) {
    const r = reasonText(e.reason);
    return e.field ? `${e.field}: ${r.toLowerCase()}` : r;
  }
  const t = titles[e.type];
  if (t) return e.connector ? `${t}: ${e.connector}` : t;
  return null;
}

/** Events about a visitor's request (the others concern a site or a connector). */
export function isRequest(e: LogEvent): boolean {
  return Boolean(e.path || e.request_id || e.src_ip);
}

const explain: Record<string, string> = {
  route_not_mapped: "The path is not in the map, so for the gateway it does not exist. If it is a real page of the site, add it to the map.",
  authz_not_owned: "A user asked for an object the site never showed them (owned_by_user). Typical of someone changing numbers in the address.",
  authz_denied: "The visitor's role may not open this entry of the map.",
  method_not_allowed: "The map does not allow this method on this address.",
  form_token_mismatch: "The form was not one the gateway served to this visitor, or it expired.",
  cross_origin: "A state-changing call came from another site: refused against CSRF.",
  legacy_unreachable: "The legacy application did not answer. Visitors saw the standard error page.",
  connector_unavailable: "The connector of this site is offline: check the server it runs on.",
  scheme_mismatch: "The legacy application sends visitors to the same page in https while the site is served in http (or the other way round), so the browser would go round in circles. Make them agree: change the site address in the application (WordPress: Settings → General) or the scheme in Site → Certificate.",
  oidc_no_role: "The user logged in with the identity provider, but is in none of the groups the map lets in.",
  oidc_invalid_token: "The identity provider's token failed the checks (signature, issuer, audience, expiry or nonce): nobody was logged in.",
  oidc_bad_callback: "A login answer did not match the login this browser started: refused against forged logins.",
  body_too_large: "The body was larger than the area allows (max_body_kb). For uploads, raise the size in a new version of the map.",
  unsupported_body: "The body is neither a form, JSON, XML, text nor a file: it cannot be rebuilt, so it never reaches the legacy site.",
  raw_type: "A file sent as the whole body did not match its declared type, or was a page, a script or a program.",
  websocket_not_supported: "The page opened a WebSocket: not supported yet. Most sites fall back to ordinary requests.",
};

const byType: Partial<Record<EventType, string>> = {
  "request.served": "Rebuilt from the legacy application and served.",
  "field.rejected": "A value broke the rules of its field in the map. The legacy application was not contacted. Values are never stored.",
  "ratelimit.hit": "Too many requests from this address for this entry. Requests are refused until the window passes.",
  "behavior.alert": "This address behaves like a scanner or a bot.",
  "legacy.error": "The legacy application failed; nothing of its error was shown to the visitor.",
  "exposure.alert": "The site still answers on its old public IP: it can be reached without Equalizer.",
  "connector.down": "The connector lost its tunnel to Equalizer.",
  "parse.failed": "A legacy page could not be rebuilt; the visitor saw an error page instead.",
};

export function explanation(e: LogEvent): string {
  return (e.reason && explain[e.reason]) || byType[e.type] || "Refused by the gateway.";
}
