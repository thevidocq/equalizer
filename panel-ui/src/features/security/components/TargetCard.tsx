import { formatDate } from "@/shared/ui";
import type { SecurityTarget } from "../hooks/useSecurity";
import { ScanButton } from "./ScanButton";
import { bySeverity, closedByMap, severityStyle, stillOpen } from "./verdicts";

const titles = { protected: "Through Equalizer", original: "Original site" };
const blurbs = {
  protected: "The site as visitors reach it: what a scan finds here is what is left open.",
  original: "The site without Equalizer in front, for a before and after. Possible only while it still answers on its own name, before isolation.",
};
const states = { pending: "Waiting to start", running: "Scanning…", completed: "Done", failed: "Failed" };

/** One target of the scans: its latest scan, the findings by severity, the button to scan. */
export function TargetCard({ app, t, canScan, configured }: { app: string; t: SecurityTarget; canScan: boolean; configured: boolean }) {
  const scan = t.scan ?? null;
  const going = scan?.status === "pending" || scan?.status === "running";
  const counts = bySeverity(t.findings);
  return (
    <section className="flex flex-col gap-3 rounded-[var(--radius-panel)] border border-rule bg-surface p-4">
      <div>
        <h2 className="text-base font-semibold">{titles[t.target]}</h2>
        <p className="font-mono text-sm">{t.domain ?? "—"}</p>
        <p className="mt-1 text-sm text-ink-soft">{blurbs[t.target]}</p>
      </div>
      {!t.available && t.reason ? <p className="text-sm text-ink-soft">Not available: {t.reason}.</p> : null}
      {scan ? (
        <div className="text-sm">
          <p>
            <span className="font-medium">{states[scan.status]}</span>
            {" · "}{scan.scan_type} scan of {formatDate(scan.created_at)}
            {scan.requested_by ? ` by ${scan.requested_by}` : ""}
          </p>
          {scan.error ? <p className="text-state-rejected">{scan.error}</p> : null}
          {scan.risk_score != null ? <p>Risk score: <span className="font-semibold">{scan.risk_score}</span>/100</p> : null}
        </div>
      ) : <p className="text-sm text-ink-soft">Never scanned.</p>}
      {counts.length ? (
        <div className="flex flex-wrap items-center gap-1.5 text-xs">
          {counts.map((c) => (
            <span key={c.severity} className={`rounded px-1.5 py-0.5 font-semibold uppercase ${severityStyle[c.severity]}`}>
              {c.count} {c.severity}
            </span>
          ))}
          <span className="ml-1 text-ink-soft">
            {t.target === "original"
              ? `${closedByMap(t.findings)} of ${t.findings.length} closed by the published map`
              : `${stillOpen(t.findings)} not closed by the map`}
          </span>
        </div>
      ) : null}
      {canScan && t.available && t.domain ? (
        <div><ScanButton app={app} target={t.target} domain={t.domain} disabled={!configured || going} /></div>
      ) : null}
    </section>
  );
}
