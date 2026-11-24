import { useState } from "react";
import { Link } from "react-router";
import { Table, type Column } from "@/shared/ui";
import type { SecurityFinding } from "../hooks/useSecurity";
import { severityStyle, shortURL, verdictOf } from "./verdicts";

const tones = { good: "text-state-published", warn: "text-state-review", plain: "text-ink-soft" };

/** The findings of a scan, the most severe first, each with what the map does with it. */
export function FindingsTable({ findings, target, caption, mapBase }: {
  findings: SecurityFinding[]; target: "protected" | "original"; caption: string; mapBase: string;
}) {
  const [open, setOpen] = useState<string | null>(null);
  const columns: Column<SecurityFinding>[] = [
    { header: "Severity", className: "w-24", cell: (f) => (
      <span className={`rounded px-1.5 py-0.5 text-xs font-semibold uppercase ${severityStyle[f.severity]}`}>{f.severity}</span>
    ) },
    { header: "Finding", cell: (f) => (
      <div className="flex flex-col gap-0.5">
        <button type="button" className="text-left font-medium hover:underline" aria-expanded={open === f.id}
          onClick={() => setOpen(open === f.id ? null : f.id)}>{f.title}</button>
        {f.url ? (
          <span className="break-all font-mono text-xs text-ink-soft">
            {f.http_method ?? "GET"} {shortURL(f.url)}{f.parameter ? ` · ${f.parameter}` : ""}
          </span>
        ) : null}
        {open === f.id ? <Detail f={f} target={target} /> : null}
      </div>
    ) },
    { header: "With Equalizer", className: "w-40", cell: (f) => {
      const v = verdictOf(f, target);
      return (
        <div className="flex flex-col">
          <span className={`font-medium ${tones[v.tone]}`} title={v.why}>{v.label}</span>
          {f.fix ? (
            <Link className="text-xs text-signal hover:underline" to={`${mapBase}/${f.fix.version}`} title={f.fix.change}>
              Closed in draft {f.fix.version}
            </Link>
          ) : null}
        </div>
      );
    } },
  ];
  return <Table rows={findings} columns={columns} rowKey={(f) => f.id} caption={caption} />;
}

function Detail({ f, target }: { f: SecurityFinding; target: "protected" | "original" }) {
  const v = verdictOf(f, target);
  return (
    <div className="mt-1 flex flex-col gap-1 text-xs">
      <p>{v.why}{f.map.entry ? ` Entry: ${f.map.entry}${f.map.field_type ? `, type ${f.map.field_type}` : ""}.` : ""}</p>
      {f.fix ? <p>Draft {f.fix.version}: {f.fix.change}.</p> : null}
      <p className="text-ink-soft">
        Found by {f.source}
        {f.cvss != null ? ` · CVSS ${f.cvss}` : ""}
        {f.cve.length ? ` · ${f.cve.join(", ")}` : ""}
        {f.cwe.length ? ` · ${f.cwe.join(", ")}` : ""}
      </p>
      {f.evidence ? <pre className="max-h-40 overflow-auto whitespace-pre-wrap break-all rounded bg-paper p-2 font-mono">{f.evidence}</pre> : null}
    </div>
  );
}
