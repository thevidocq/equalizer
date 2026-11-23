import { Link } from "react-router";
import { formatDate } from "@/shared/ui";
import { kindOf, reasonText, type EventType } from "@/features/logs";
import type { Schemas } from "@/shared/api/client";

type Props = { tenant: string; app: string; card: Schemas["AssistantEventCard"] };

/** One event of the logs, as the Logs page shows it, with the way there (a new tab: the conversation stays). */
export function EventCard({ tenant, app, card }: Props) {
  const kind = kindOf(card.type as EventType);
  const q = new URLSearchParams({ app, q: card.request_id, since: "30d" });
  return (
    <div className="rounded-[var(--radius-control)] border border-rule bg-surface p-3 text-[0.8125rem]">
      <p className="text-ink-soft">{formatDate(card.ts)} · request <span className="font-mono">{card.request_id}</span></p>
      <p className="mt-1 flex items-center gap-2 font-semibold text-ink">
        <span aria-hidden="true" className="inline-block h-2 w-2 rounded-full" style={{ background: kind.color }} />
        {kind.label}{card.status ? ` · ${card.status}` : ""}
      </p>
      {card.path ? <p className="mt-1 font-mono break-all text-ink">{card.method} {card.path}</p> : null}
      {card.reason ? (
        <p className="mt-1 text-ink-soft">
          Rule: {card.route ? `${card.route} · ` : ""}{card.field ? `${card.field}: ` : ""}{reasonText(card.reason).toLowerCase()}
        </p>
      ) : null}
      <div className="mt-2 flex items-center justify-between gap-2">
        <span className="text-ink-soft">Visitor <span className="font-mono">{card.visitor || "—"}</span></span>
        <Link to={`/t/${tenant}/logs?${q.toString()}`} target="_blank" rel="noopener" className="font-semibold text-signal hover:underline">Open in Logs</Link>
      </div>
    </div>
  );
}
