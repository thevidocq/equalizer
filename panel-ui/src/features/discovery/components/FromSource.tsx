import { Notice } from "@/shared/ui";
import type { Schemas } from "@/shared/api/client";
import { byOwner } from "./owner";
import { entryLine } from "./entry";

type S = Schemas["StudySource"];

const RISK: Record<string, string> = {
  sql: "SQL query", eval: "eval", include: "file included", exec: "command run", unserialize: "unserialize", file_write: "file written",
};

/** A list grouped by extension, each group closed but its count. */
function Grouped<T>({ title, items, total, file, named, line }: {
  title: string; items: T[]; total?: number; file: (i: T) => string; named: (i: T) => string | null | undefined; line: (i: T) => string;
}) {
  if (!items.length) return null;
  const count = total && total > items.length ? `first ${items.length} of ${total}` : `${items.length}`;
  return (
    <div className="flex flex-col gap-1">
      <h4 className="text-sm font-semibold">{title} ({count})</h4>
      {byOwner(items, file, named).map(([who, list]) => (
        <details key={who} className="text-sm">
          <summary className="cursor-pointer">{who} ({list.length})</summary>
          <ul className="max-h-56 overflow-auto pl-4 font-mono text-[12.5px] leading-relaxed">{list.map((i, n) => <li key={n}>{line(i)}</li>)}</ul>
        </details>
      ))}
    </div>
  );
}

const fileOf = (where?: string | null) => (where ?? "").replace(/:\d+$/, "").replace(/^\//, "");

/** What the site's source code added to the discovery. */
export function FromSource({ source }: { source?: S }) {
  if (!source) return null;
  if (source.status !== "used") {
    return <Notice tone="error">The source code was not used{source.status === "late" ? " (the read took more than 10 minutes)" : ""}: {source.error}</Notice>;
  }
  const known = source.known ? `; ${source.known} files of the core and of libraries are known code, only their versions read` : "";
  return (
    <section className="flex flex-col gap-3">
      <h3 className="text-sm font-semibold">From the source code</h3>
      <p className="text-sm text-ink-soft">
        {source.files ?? 0} code files studied ({source.stack}){known}. Only the extensions' and the site's own code is studied.
        Names, types and places only: the code stayed on your network.
      </p>
      <Grouped title="Fields typed by the code" items={source.typed ?? []} file={(t) => fileOf(t.where)} named={(t) => t.owner}
        line={(t) => `${t.route}  ${t.field} → ${t.type}${t.where ? `  (${t.where})` : ""}`} />
      <Grouped title="Entry points the links do not show (served by the site area)" items={source.unseen ?? []} total={source.unseen_total}
        file={(u) => fileOf(u.where) || u.path} named={(u) => u.owner} line={entryLine} />
      <Grouped title="Calls where a request value may do harm" items={source.risks ?? []} total={source.risks_total} file={(r) => fileOf(r.where)}
        named={(r) => r.owner}
        line={(r) => `${r.where}  ${RISK[r.kind] ?? r.kind}${r.input ? ` ← ${r.input}` : ""}${r.route ? `  (${r.route})` : ""}`} />
    </section>
  );
}
