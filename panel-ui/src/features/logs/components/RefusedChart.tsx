import { formatTick } from "@/shared/ui";
type Bucket = { ts: string; count: number };

const labels: Record<number, string> = { 300: "5 minutes", 3600: "hour", 21600: "6 hours", 86400: "day" };

function time(ts: string, step: number) {
  return ts ? formatTick(new Date(ts), step >= 86400) : "";
}

type Props = { buckets: Bucket[]; step: number; picked?: string | null; onPick?: (b: Bucket) => void };

/**
 * Refused requests per interval: the latest interval stands out. A bar is a
 * button: it shows the refused events of its interval.
 */
export function RefusedChart({ buckets, step, picked, onPick }: Props) {
  const peak = Math.max(1, ...buckets.map((b) => b.count));
  const total = buckets.reduce((s, b) => s + b.count, 0);
  const ticks = [0, Math.floor(buckets.length / 2), buckets.length - 1];
  return (
    <section aria-label="Refused requests over time" className="rounded-[var(--radius-panel)] border border-rule-soft bg-surface px-4 pb-2.5 pt-3.5">
      <div className="mb-2 flex justify-between text-[13px] text-ink-soft">
        <span className="font-semibold text-ink">Refused per {labels[step] ?? "interval"}</span>
        <span className="tabular-nums">{total.toLocaleString("en")} in the period</span>
      </div>
      <div className="flex h-16 items-end gap-[3px]" role="group"
        aria-label={`Refused requests per ${labels[step] ?? "interval"}, highest ${peak}`}>
        {buckets.map((b, i) => (
          <button key={b.ts} type="button" disabled={!b.count || !onPick} onClick={() => onPick?.(b)}
            title={`${time(b.ts, step)}: ${b.count} refused${b.count ? " (show them)" : ""}`}
            aria-label={`${time(b.ts, step)}: ${b.count} refused`} aria-pressed={picked === b.ts}
            className="min-w-1 flex-1 rounded-t-[2px] enabled:cursor-pointer enabled:hover:opacity-80"
            style={{ height: `${Math.max(b.count ? 4 : 1, Math.round((b.count / peak) * 100))}%`,
              opacity: picked && picked !== b.ts ? 0.35 : 1,
              background: picked === b.ts || (!picked && i === buckets.length - 1) ? "#b42318" : b.count ? "#e4a39c" : "#e3e8ed" }} />
        ))}
      </div>
      {buckets.length ? (
        <div className="mt-1 flex justify-between text-xs tabular-nums text-ink-soft">
          {ticks.map((i) => <span key={i}>{time(buckets[i]?.ts ?? "", step)}</span>)}
        </div>
      ) : null}
    </section>
  );
}
