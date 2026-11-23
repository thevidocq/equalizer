import { Link } from "react-router";
import type { Schemas } from "@/shared/api/client";

type Props = { tenant: string; app: string; card: Schemas["AssistantRouteCard"] };

/** An entry of the map and what it checks on each value. */
export function RouteCard({ tenant, app, card }: Props) {
  return (
    <div className="rounded-[var(--radius-control)] border border-rule bg-surface p-3 text-[0.8125rem]">
      <p className="flex justify-between gap-2">
        <span className="font-semibold text-ink">{card.id}</span>
        <Link to={`/t/${tenant}/sites/${app}/maps/${card.version}`} className="text-signal hover:underline">Map v{card.version}</Link>
      </p>
      <p className="mt-1 font-mono break-all text-ink">{card.method} {card.path}</p>
      {card.checks.length ? (
        <ul className="mt-2 space-y-1">
          {card.checks.map((c, i) => (
            <li key={i}>
              <span className="font-mono text-ink">{c.where === "query" ? `?${c.name}` : c.where === "path" ? `{${c.name}}` : c.name}</span>
              {c.where.startsWith("form") ? <span className="text-ink-soft"> ({c.where})</span> : null}
              <span className="text-ink-soft"> · {c.rule}</span>
            </li>
          ))}
        </ul>
      ) : <p className="mt-1 text-ink-soft">No value to check.</p>}
    </div>
  );
}
