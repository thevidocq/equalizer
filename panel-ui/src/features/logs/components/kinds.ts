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
};

/** A short line for the table: the field name first when there is one. */
export function reasonLabel(e: LogEvent): string {
  const r = e.reason ? reasons[e.reason] ?? e.reason.replaceAll("_", " ") : kindOf(e.type).label;
  return e.field ? `${e.field}: ${r.toLowerCase()}` : r;
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
  oidc_no_role: "The user logged in with the identity provider, but is in none of the groups the map lets in.",
  oidc_invalid_token: "The identity provider's token failed the checks (signature, issuer, audience, expiry or nonce): nobody was logged in.",
  oidc_bad_callback: "A login answer did not match the login this browser started: refused against forged logins.",
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
