import type { AuditEntry } from "@/shared/api/client";

const verbs: Record<string, string> = {
  "auth.login": "logged in",
  "auth.logout": "logged out",
  "auth.login_failed": "failed to log in",
  "auth.mfa_enabled": "set up an authenticator app:",
  "auth.mfa_verified": "completed the second login step",
  "auth.mfa_failed": "entered a wrong second-step code",
  "auth.password_changed": "changed the password of",
  "user.deactivate": "deactivated",
  "user.mfa_reset": "reset the authenticator of user",
  "tenant.create": "added customer",
  "tenant.reinvite": "invited again the administrator of",
  "user.create": "added",
  "user.invite": "invited",
  "user.invite_revoked": "revoked the invitation of",
  "user.join": "joined with the invitation for",
  "app.create": "protected site",
  "app.legacy_ip": "changed the old IP of",
  "site.exposure": "exposure check changed for",
  "connector.create": "added connector",
  "connector.rotate": "rotated the token of",
  "connector.delete": "removed connector",
  "map.draft": "saved a draft of",
  "map.submit": "sent for review",
  "map.approve": "approved",
  "map.reject": "rejected",
  "map.publish": "published",
};

/** "ap@acme.test approved version 3 of portal". */
export function describeEntry(e: AuditEntry): string {
  const who = e.user ?? "Someone";
  const verb = verbs[e.action] ?? e.action;
  const version = typeof e.detail?.version === "number" ? `version ${e.detail.version} of ` : "";
  const target = e.target ? ` ${version}${e.target}` : "";
  return `${who} ${verb}${target}`.trim();
}
