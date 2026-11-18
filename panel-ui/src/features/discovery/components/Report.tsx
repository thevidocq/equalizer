import { Link } from "react-router";
import { Notice } from "@/shared/ui";
import type { Discovery } from "../hooks/useDiscovery";

type R = Record<string, any>; // eslint-disable-line @typescript-eslint/no-explicit-any

function Stat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="border border-rule bg-surface px-4 py-3">
      <div className="text-xs text-ink-soft">{label}</div>
      <div className="text-lg font-semibold">{value}</div>
    </div>
  );
}

function List({ title, items, render }: { title: string; items?: R[]; render: (i: R) => string }) {
  if (!items?.length) return null;
  return (
    <section className="flex flex-col gap-1">
      <h3 className="text-sm font-semibold">{title} ({items.length})</h3>
      <ul className="max-h-56 overflow-auto font-mono text-[12.5px] leading-relaxed">
        {items.map((i, n) => <li key={n}>{render(i)}</li>)}
      </ul>
    </section>
  );
}

/** What a finished discovery found and proposes. */
export function Report({ d }: { d: Discovery }) {
  const r = (d.report ?? {}) as R;
  if (d.status === "failed") return <Notice tone="error">The discovery failed: {d.error}</Notice>;
  if (d.status !== "done") return <p className="text-sm text-ink-soft">Running: {(d.progress as R)?.pages ?? 0} pages so far.</p>;
  const ai = (r.ai ?? {}) as R;
  return (
    <div className="flex flex-col gap-5">
      {d.draft_version ? (
        <Notice tone="info">A draft was proposed: <Link className="font-semibold text-signal hover:underline" to={`../maps/${d.draft_version}`}>
          version {d.draft_version}</Link>. Review it and send it for approval.</Notice>
      ) : <Notice tone="info">The map already covers what was found: no draft needed.</Notice>}
      <div className="grid gap-3 sm:grid-cols-4">
        <Stat label="Built with" value={r.cms ? `${r.cms}${r.version ? " " + r.version : ""}` : "Not recognised"} />
        <Stat label="Coverage" value={`${r.coverage ?? 0}%`} />
        <Stat label="Pages found" value={r.pages_found ?? 0} />
        <Stat label="Kept out" value={(r.excluded ?? []).length} />
      </div>
      {r.known_issues?.length ? (
        <Notice tone="error">{r.known_issues.length} component(s) with known issues: their paths stay closed. Update them.</Notice>
      ) : null}
      {ai.used === false ? <p className="text-sm text-ink-soft">Without AI{ai.reason ? `: ${ai.reason}` : ai.error ? `: ${ai.error}` : ""}.</p> : null}
      {ai.notes?.length ? <List title="AI notes" items={ai.notes.map((n: string) => ({ n }))} render={(i) => i.n} /> : null}
      <List title="Routes added" items={r.routes_added} render={(i) => `${i.path}${i.logged_in_only ? "  (logged in)" : ""}`} />
      <List title="Forms added" items={r.forms_added} render={(i) => `${i.route}  ${i.fields.join(", ")}`} />
      <List title="Protection against IDOR proposed" items={r.issues_proposed}
        render={(i) => `${i.list} → ${i.detail}  (${i.field}${i.roles?.length ? `, owned_by_user for ${i.roles.join(", ")}` : ""})`} />
      <List title="Kept out of the map" items={r.excluded} render={(i) => `${i.path}  ${i.reason}`} />
      <List title="Asked by visitors, still missing" items={r.visitors_missing} render={(i) => `${i.path}  (${i.clients} visitors)`} />
      <List title="Not supported yet" items={r.not_supported} render={(i) => `${i.path}  ${i.reason}`} />
      <List title="In the map, not seen any more" items={(r.not_seen_anymore ?? []).map((p: string) => ({ p }))} render={(i) => i.p} />
      <List title="Components" items={r.components} render={(i) => `${i.name}${i.version ? " " + i.version : ""}`} />
    </div>
  );
}
