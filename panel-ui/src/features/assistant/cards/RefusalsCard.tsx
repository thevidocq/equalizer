import { reasonText } from "@/features/logs";
import type { Schemas } from "@/shared/api/client";

const periods: Record<string, string> = { "1h": "last hour", "24h": "last 24 hours", "7d": "last 7 days", "30d": "last 30 days" };
const kinds: Record<string, string> = { "request.blocked": "Blocked", "field.rejected": "Field refused", "ratelimit.hit": "Rate limit" };

/** The refusals of a period by reason: requests and distinct visitors. */
export function RefusalsCard({ card }: { card: Schemas["AssistantRefusalsCard"] }) {
  return (
    <div className="rounded-[var(--radius-control)] border border-rule bg-surface p-3 text-[0.8125rem]">
      <p className="font-semibold text-ink">Refused · {periods[card.since] ?? card.since}</p>
      {card.rows.length ? (
        <table className="mt-2 w-full text-left">
          <thead className="text-ink-soft">
            <tr><th className="py-1 font-medium">Reason</th><th className="py-1 text-right font-medium">Requests</th><th className="py-1 text-right font-medium">Visitors</th></tr>
          </thead>
          <tbody>
            {card.rows.map((r, i) => (
              <tr key={i} className="border-t border-rule-soft">
                <td className="py-1 pr-2">{r.reason ? reasonText(r.reason) : (kinds[r.type] ?? r.type)}</td>
                <td className="py-1 text-right tabular-nums">{r.requests}</td>
                <td className="py-1 text-right tabular-nums">{r.visitors}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : <p className="mt-1 text-ink-soft">Nothing refused.</p>}
      <p className="mt-2 text-xs text-ink-soft">
        {card.partial ? `The latest ${card.total} events only: Logs has them all.` : `${card.total} in all.`}
      </p>
    </div>
  );
}
