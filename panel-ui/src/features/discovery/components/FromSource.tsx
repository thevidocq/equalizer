import { Notice } from "@/shared/ui";
import type { Schemas } from "@/shared/api/client";
import { byOwner } from "./owner";

type S = Schemas["StudySource"];

const RISK: Record<string, string> = {
  sql: "SQL query", eval: "eval", include: "file included", exec: "command run", unserialize: "unserialize", file_write: "file written",
};
const ORIGIN: Record<string, string> = {
  rest_route: "REST", wp_ajax: "AJAX (logged in)", wp_ajax_nopriv: "AJAX (anyone)", admin_post: "admin-post", php_file: "PHP file",
};

/** A list grouped by plugin or theme, each group closed but its count. */
function Grouped<T>({ title, items, total, file, line }: { title: string; items: T[]; total?: number; file: (i: T) => string; line: (i: T) => string }) {
  if (!items.length) return null;
  const count = total && total > items.length ? `first ${items.length} of ${total}` : `${items.length}`;
  return (
    <div className="flex flex-col gap-1">
      <h4 className="text-sm font-semibold">{title} ({count})</h4>
      {byOwner(items, file).map(([who, list]) => (
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
  return (
    <section className="flex flex-col gap-3">
      <h3 className="text-sm font-semibold">From the source code</h3>
      <p className="text-sm text-ink-soft">
        {source.files ?? 0} code files read ({source.stack}). WordPress's core and the libraries plugins ship are left out: only
        the plugins', themes' and site's own code is studied. Names, types and places only: the code stayed on your network.
      </p>
      <Grouped title="Fields typed by the code" items={source.typed ?? []} file={(t) => fileOf(t.where)}
        line={(t) => `${t.route}  ${t.field} → ${t.type}${t.where ? `  (${t.where})` : ""}`} />
      <Grouped title="Entry points the links do not show (served by the site area)" items={source.unseen ?? []} total={source.unseen_total}
        file={(u) => fileOf(u.where) || u.path} line={(u) => `${u.path}${u.action ? `?action=${u.action}` : ""}  ${ORIGIN[u.origin] ?? u.origin}${u.public ? ", no login" : ""}`} />
      <Grouped title="Calls where a request value may do harm" items={source.risks ?? []} total={source.risks_total} file={(r) => fileOf(r.where)}
        line={(r) => `${r.where}  ${RISK[r.kind] ?? r.kind}${r.input ? ` ← ${r.input}` : ""}${r.route ? `  (${r.route})` : ""}`} />
    </section>
  );
}
