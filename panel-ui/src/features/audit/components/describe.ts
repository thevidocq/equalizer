import type { AuditEntry } from "@/shared/api/client";

type Detail = Record<string, unknown>;
type Say = (target: string, d: Detail) => string;

const version = (d: Detail) => (typeof d.version === "number" ? `version ${d.version} of the map of ` : "the map of ");
const schemes: Record<string, string> = {
  https: "HTTPS served by Equalizer",
  https_proxy: "HTTPS handled by a proxy in front",
  http: "plain HTTP",
};

/** What each action did, as the end of a sentence. */
const actions: Record<string, Say> = {
  "auth.login": () => "logged in",
  "auth.logout": () => "logged out",
  "auth.login_failed": () => "failed to log in",
  "auth.mfa_enabled": () => "set up an authenticator app",
  "auth.mfa_verified": () => "completed the second login step",
  "auth.mfa_failed": () => "entered a wrong second-step code",
  "auth.password_changed": () => "changed their password",
  "auth.password_reset_requested": (t) => `asked for a password reset for ${t}`,
  "auth.password_reset": () => "set a new password with a reset link",
  "user.deactivate": (t) => `deactivated ${t}`,
  "user.mfa_reset": () => "reset the authenticator app of a user",
  "user.create": (t) => `added ${t}`,
  "user.invite": (t) => `invited ${t}`,
  "user.invite_revoked": () => "revoked an invitation",
  "user.join": (t) => `joined as ${t}`,
  "tenant.create": (t) => `added the customer ${t}`,
  "tenant.delete": (t) => `deleted the customer ${t}`,
  "tenant.reinvite": (t) => `invited again the administrator of ${t}`,
  "app.create": (t) => `added the site ${t}`,
  "app.legacy_ip": (t) => `changed the address of the legacy server of ${t}`,
  "app.aliases": (t) => `changed the other names of ${t}`,
  "app.certificate": (t) => `uploaded a certificate for ${t}`,
  "app.certificate_removed": (t) => `removed the certificate of ${t}`,
  "app.public_scheme": (t, d) => `set ${t} to ${schemes[String(d.scheme)] ?? String(d.scheme)}`,
  "app.reach": (t, d) => `changed how Equalizer reaches ${t} (${d.mode === "direct" ? "directly" : "through a connector"})`,
  "app.never_public": (t, d) =>
    d.never_public ? `marked ${t} as never reachable from the internet` : `let ${t} be checked from the internet again`,
  "oidc.secret_set": (t) => `saved the identity provider secret of ${t}`,
  "oidc.secret_deleted": (t) => `removed the identity provider secret of ${t}`,
  "llm.set": (t) => `set the AI model (${t})`,
  "llm.delete": () => "removed the AI model",
  "scanner.set": (_t, d) => `connected the security scanner${d.key_changed ? " with a new key" : ""}`,
  "scanner.delete": () => "disconnected the security scanner",
  "security.scan": (t, d) => d.target === "original"
    ? `started a ${String(d.scan_type)} scan of the original site of ${t} (${String(d.domain)}), confirming it is theirs or allowed in writing`
    : `started a ${String(d.scan_type)} security scan of ${t} through Equalizer`,
  "security.scan_completed": (t, d) => `finished the security scan of ${t}${d.target === "original" ? " (original site)" : ""}: ${String(d.findings)} found`,
  "security.draft": (t, d) => `built draft version ${String(d.version)} of ${t} from a security scan: it closes ${String(d.closes)} finding(s)`,
  "security.scan_failed": (t, d) => `could not finish the security scan of ${t}: ${String(d.reason)}`,
  "assistant.asked": (t, d) => `asked the assistant about ${t}` +
    (typeof d.proposals === "number" && d.proposals > 0 ? ` (${d.proposals} change${d.proposals > 1 ? "s" : ""} proposed)` : ""),
  "assistant.settings": (_t, d) => `set the assistant to keep conversations for ${String(d.keep_days)} days`,
  "assistant.draft": (t, d) => `made a change proposed by the assistant into ${version(d)}${t}`,
  "discovery.start": (t) => `started a discovery of ${t}`,
  "discovery.finish": (t, d) =>
    d.error ? `could not finish the discovery of ${t}` :
      typeof d.draft_version === "number" ? `finished the discovery of ${t}: draft version ${d.draft_version} is ready to review` :
        `finished the discovery of ${t}: nothing new`,
  "discovery.schedule": (t, d) => `set the continuous discovery of ${t} to ${when(d)}`,
  "site.exposure": (t, d) => (d.status === "exposed" ? `found ${t} still reachable without Equalizer` : `checked that ${t} is reachable only through Equalizer`),
  "connector.create": (t) => `added the connector ${t}`,
  "connector.rotate": (t) => `changed the token of the connector ${t}`,
  "connector.delete": (t) => `removed the connector ${t}`,
  "map.draft": (t, d) => `saved a draft: ${version(d)}${t}`,
  "map.submit": (t, d) => `sent ${version(d)}${t} for review`,
  "map.approve": (t, d) => `approved ${version(d)}${t}`,
  "map.reject": (t, d) => `discarded ${version(d)}${t}`,
  "map.publish": (t, d) => `published ${version(d)}${t}`,
  "map.rollback": (t, d) => `went back to ${version(d)}${t}`,
  "map.verify": (t, d) => `asked to check ${version(d)}${t}`,
  "map.corrected": (t, d) => `corrected version ${String(d.from)} of the map of ${t} into version ${String(d.version)}`,
  "map.superseded": (t, d) => `replaced draft ${String(d.from)} of the map of ${t} by its corrected version ${String(d.version)}`,
  "map.adopted": (t, d) =>
    `added to the map of ${t}, in version ${String(d.version)}, a ${String(d.kind)} visitors asked for (discovery #${String(d.discovery)})`,
  "map.correction_withdrawn": (t, d) =>
    `withdrew its correction ${String(d.version)} of the map of ${t}: version ${String(d.from)} was taken on by a person`,
  "logs.export": (t) => (t ? `exported the logs of ${t}` : "exported the logs"),
  "settings.alerts": (_, d) => (d.enabled ? "turned alert emails on or changed who gets them" : "turned alert emails off"),
  "alert.sent": (_, d) => alertSentence(d),
};

/** Actions Equalizer takes by itself; the others without a person were done by the agent. */
const byEqualizer = new Set(["alert.sent", "site.exposure", "security.scan_completed", "security.scan_failed", "security.draft", "map.corrected", "map.superseded", "map.correction_withdrawn"]);
const byAgent = new Set(["discovery.finish", "discovery.start", "map.verify"]);

const weekdays = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];

function when(d: Detail): string {
  if (d.schedule === "off") return "off";
  const at = typeof d.hour === "number" ? ` at ${String(d.hour).padStart(2, "0")}:00${d.zone ? ` (${String(d.zone)})` : ""}` : "";
  const day = d.schedule === "weekly" ? `every ${typeof d.weekday === "number" ? weekdays[d.weekday] ?? "week" : "week"}` : "every day";
  return `${day}${at}`;
}

type Alert = { site?: string; check?: string; down?: boolean; detail?: string };

function alertSentence(d: Detail): string {
  const alerts = Array.isArray(d.alerts) ? (d.alerts as Alert[]) : [];
  const to = Array.isArray(d.to) ? (d.to as string[]) : [];
  const what = alerts.length
    ? alerts.map((a) => a.check === "Draft ready"
      ? `draft ${a.detail ?? ""} of ${a.site ?? "a site"} ready to publish`
      : `${a.check ?? "check"} ${a.down ? "failing" : "back to normal"} on ${a.site ?? "a site"}`).join("; ")
    : `${String(d.changes ?? "some")} change(s)`;
  const who = to.length ? to.join(", ") : `${String(d.recipients ?? "")} address(es)`.trim();
  return `emailed ${who}: ${what}`;
}

/** Who did it: the person, or Equalizer and its agent when nobody did. */
export function actor(e: AuditEntry): string {
  if (e.user) return e.user;
  if (byEqualizer.has(e.action)) return "Equalizer";
  if (byAgent.has(e.action)) return "The agent";
  return "Someone";
}

/** True when the entry is a problem worth seeing at a glance. */
export function isWarning(e: AuditEntry): boolean {
  const d = e.detail ?? {};
  return e.action === "auth.login_failed" || e.action === "auth.mfa_failed" ||
    (e.action === "site.exposure" && d.status === "exposed") ||
    (e.action === "discovery.finish" && Boolean(d.error)) || e.action === "security.scan_failed" ||
    (e.action === "alert.sent" && Array.isArray(d.alerts) && (d.alerts as Alert[]).some((a) => a.down));
}

/** "ap@acme.test published version 3 of the map of portal". */
export function describeEntry(e: AuditEntry): string {
  const say = actions[e.action];
  const what = say ? say(e.target, e.detail ?? {}) : `${e.action.replace(/[._]/g, " ")} ${e.target}`.trim();
  return `${actor(e)} ${what}`;
}
