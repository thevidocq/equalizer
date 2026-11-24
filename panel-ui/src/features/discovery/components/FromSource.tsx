import { Notice } from "@/shared/ui";
import type { Schemas } from "@/shared/api/client";

type S = Schemas["StudySource"];

const RISK: Record<string, string> = {
  sql: "SQL query", eval: "eval", include: "file included", exec: "command run", unserialize: "unserialize", file_write: "file written",
};
const ORIGIN: Record<string, string> = {
  rest_route: "REST", wp_ajax: "AJAX (logged in)", wp_ajax_nopriv: "AJAX (anyone)", admin_post: "admin-post", php_file: "PHP file",
};

function Lines({ title, items }: { title: string; items: string[] }) {
  if (!items.length) return null;
  return (
    <div className="flex flex-col gap-1">
      <h4 className="text-sm font-semibold">{title} ({items.length})</h4>
      <ul className="max-h-56 overflow-auto font-mono text-[12.5px] leading-relaxed">{items.map((t, n) => <li key={n}>{t}</li>)}</ul>
    </div>
  );
}

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
        {source.files ?? 0} code files read ({source.stack}), {source.routes ?? 0} entry points. Names, types and places only: the code stayed on your network.
      </p>
      <Lines title="Fields typed by the code" items={(source.typed ?? []).map((t) => `${t.route}  ${t.field} → ${t.type}${t.where ? `  (${t.where})` : ""}`)} />
      <Lines title="Entry points the links do not show (served by the site area)"
        items={(source.unseen ?? []).map((u) => `${u.path}${u.action ? `?action=${u.action}` : ""}  ${ORIGIN[u.origin] ?? u.origin}${u.public ? ", no login" : ""}`)} />
      <Lines title="Calls where a request value may do harm"
        items={(source.risks ?? []).map((r) => `${r.where}  ${RISK[r.kind] ?? r.kind}${r.input ? ` ← ${r.input}` : ""}${r.route ? `  (${r.route})` : ""}`)} />
      <Lines title="Components in the code" items={(source.components ?? []).map((c) => `${c.name}${c.version ? " " + c.version : ""}`)} />
    </section>
  );
}
