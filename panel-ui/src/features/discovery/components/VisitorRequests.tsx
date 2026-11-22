type R = Record<string, any>; // eslint-disable-line @typescript-eslint/no-explicit-any

function label(i: R): string {
  return i.kind === "key" ? `${i.route}  ?${i.key}=` : String(i.path ?? "");
}

/**
 * What visitors asked for that the agent checked on the site and put in its
 * draft (a file type, a page option with the values the site offers): marked
 * so that whoever publishes sees it, and can take it out of the draft first.
 */
export function VisitorRequests({ report }: { report: R }) {
  const items: R[] = Array.isArray(report.visitors_added) ? report.visitors_added : [];
  if (!items.length) return null;
  return (
    <section className="flex flex-col gap-1">
      <h3 className="text-sm font-semibold">Added from visitors' requests ({items.length})</h3>
      <p className="text-xs text-ink-soft">
        Checked on the site by the agent and put in its draft: review them before publishing.
      </p>
      <ul>
        {items.map((i, n) => (
          <li key={n} className="flex min-w-0 flex-col border-b border-rule py-2">
            <span className="break-all font-mono text-[12.5px]">{label(i)}</span>
            <span className="text-xs text-ink-soft">{String(i.why ?? "")}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}
