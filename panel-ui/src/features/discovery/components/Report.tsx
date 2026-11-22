import { useParams } from "react-router";
import { Notice, Spinner } from "@/shared/ui";
import { STILL_OPEN } from "@/features/maps";
import { DraftNotice } from "./DraftNotice";
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

const LOGIN: Record<string, string> = { ok: "logged in", failed: "login failed", no_login_form: "no login form found" };

/** What a finished discovery found and proposes. */
export function Report({ d }: { d: Discovery }) {
  const { app = "" } = useParams();
  const r = (d.report ?? {}) as R;
  if (d.status === "failed") return <Notice tone="error">The discovery failed: {d.error}</Notice>;
  if (d.status !== "done") {
    const pages = (d.progress as R)?.pages ?? 0;
    return (
      <p className="text-sm text-ink-soft">
        <Spinner label={d.status === "queued" ? "Waiting to start…" : `Running: ${pages} ${pages === 1 ? "page" : "pages"} so far…`} />
      </p>
    );
  }
  const ai = (r.ai ?? {}) as R;
  return (
    <div className="flex flex-col gap-5">
      {d.draft_version ? <DraftNotice app={app} draft={d.draft_version} />
        : <Notice tone="info">The map already covers what was found: no draft needed.</Notice>}
      <div className="grid gap-3 sm:grid-cols-4">
        <Stat label="Built with" value={r.cms ? `${r.cms}${r.version ? " " + r.version : ""}` : "Not recognised"} />
        <Stat label="Coverage" value={`${r.coverage ?? 0}%`} />
        <Stat label="Pages found" value={r.pages_found ?? 0} />
        <Stat label="Kept out" value={(r.excluded ?? []).length} />
      </div>
      {r.known_issues?.length ? (
        <Notice tone="error">{r.known_issues.length} component(s) with known issues: their paths stay closed. Update them.</Notice>
      ) : null}
      {ai.used === false && ai.error ? (
        <Notice tone="error">The AI was asked but not used, so the draft comes from the fixed rules only: {ai.error}.</Notice>
      ) : ai.used === false ? <p className="text-sm text-ink-soft">Without AI{ai.reason ? `: ${ai.reason}` : ""}.</p> : null}
      {ai.notes?.length ? <List title="AI notes" items={ai.notes.map((n: string) => ({ n }))} render={(i) => i.n} /> : null}
      <List title="Routes added" items={r.routes_added} render={(i) => `${i.path}${i.logged_in_only ? "  (logged in)" : ""}`} />
      <List title="Forms added" items={r.forms_added} render={(i) => `${i.route}  ${i.fields.join(", ")}`} />
      <List title="Protection against IDOR proposed" items={r.issues_proposed}
        render={(i) => `${i.list} → ${i.detail}  (${i.field}${i.roles?.length ? `, owned_by_user for ${i.roles.join(", ")}` : ""})`} />
      <List title="Test accounts" items={r.accounts} render={(i) => `${i.label}  ${LOGIN[i.login] ?? i.login}`} />
      {r.open ? (
        <List title="Still open" items={r.open} render={(i) => `${i.target}  ${STILL_OPEN[i.reason] ?? i.reason}${i.detail ? ` (${i.detail})` : ""}`} />
      ) : (
        <List title="Not opened by the agent (still served, by the site area)" items={r.excluded} render={(i) => `${i.path}  ${i.reason}`} />
      )}
      <List title="Asked by visitors, still missing" items={r.visitors_missing} render={(i) => `${i.path}  (${i.clients} visitors)`} />
      <List title="Served by the site area, without typed checks" items={r.open ? [] : r.not_supported} render={(i) => `${i.path}  ${i.reason}`} />
      <List title="In the map, not seen any more" items={(r.not_seen_anymore ?? []).map((p: string) => ({ p }))} render={(i) => i.p} />
      <List title="Components" items={r.components} render={(i) => `${i.name}${i.version ? " " + i.version : ""}`} />
    </div>
  );
}
