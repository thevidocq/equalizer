import type { AuditEntry } from "@/shared/api/client";

const verbs: Record<string, string> = {
  "auth.login": "logged in",
  "auth.logout": "logged out",
  "auth.login_failed": "failed to log in",
  "tenant.create": "added tenant",
  "user.create": "added",
  "app.create": "protected site",
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
