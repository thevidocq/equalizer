import { Fragment } from "react";
import { Link } from "react-router";
import { parse, type Inline } from "./parse";

function Spans({ c }: { c: Inline[] }) {
  return (
    <>
      {c.map((x, i) => {
        switch (x.t) {
          case "code": return <code key={i} className="rounded bg-paper px-1 py-0.5 font-mono text-[0.8125rem] break-all">{x.v}</code>;
          case "bold": return <strong key={i} className="font-semibold"><Spans c={x.c} /></strong>;
          case "em": return <em key={i}><Spans c={x.c} /></em>;
          case "link": return <Link key={i} to={x.to} className="text-signal hover:underline">{x.v}</Link>;
          default: return <Fragment key={i}>{x.v.split("\n").map((l, j) => (j ? [<br key={j} />, l] : l))}</Fragment>;
        }
      })}
    </>
  );
}

/** The assistant's answer, as Markdown rendered by React: never HTML. */
export function Markdown({ text }: { text: string }) {
  return (
    <div className="flex flex-col gap-2">
      {parse(text).map((b, i) => {
        switch (b.t) {
          case "h": return <p key={i} className="font-semibold text-ink"><Spans c={b.c} /></p>;
          case "ul": return <ul key={i} className="ml-4 list-disc space-y-1">{b.items.map((it, j) => <li key={j}><Spans c={it} /></li>)}</ul>;
          case "ol": return <ol key={i} className="ml-4 list-decimal space-y-1">{b.items.map((it, j) => <li key={j}><Spans c={it} /></li>)}</ol>;
          case "code": return <pre key={i} className="overflow-x-auto rounded bg-paper p-2 font-mono text-[0.8125rem]">{b.v}</pre>;
          case "table": return (
            <div key={i} className="overflow-x-auto">
              <table className="w-full text-left text-[0.8125rem]">
                <thead><tr>{b.head.map((h, j) => <th key={j} className="border-b border-rule px-2 py-1 font-semibold"><Spans c={h} /></th>)}</tr></thead>
                <tbody>{b.rows.map((r, j) => (
                  <tr key={j}>{r.map((c, k) => <td key={k} className="border-b border-rule-soft px-2 py-1"><Spans c={c} /></td>)}</tr>
                ))}</tbody>
              </table>
            </div>
          );
          default: return <p key={i}><Spans c={b.c} /></p>;
        }
      })}
    </div>
  );
}
