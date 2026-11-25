import { formatTick } from "@/shared/ui";

export type Bucket = { ts: string; count: number };
/** Which bars: what the gateway served, or what it refused. */
export type Series = "served" | "refused";
export type Picked = { ts: string; series: Series } | null;

const labels: Record<number, string> = { 300: "5 minutes", 3600: "hour", 21600: "6 hours", 86400: "day" };
const colors: Record<Series, { strong: string; soft: string }> = {
  served: { strong: "#2b7a4b", soft: "#9cc9ac" },
  refused: { strong: "#b42318", soft: "#e4a39c" },
};

function time(ts: string, step: number) {
  return ts ? formatTick(new Date(ts), step >= 86400) : "";
}

type BarsProps = { series: Series; buckets: Bucket[]; step: number; picked: Picked; onPick?: (b: Bucket, s: Series) => void };

/** One row of bars, scaled to its own highest: served requests outnumber refused ones. */
function Bars({ series, buckets, step, picked, onPick }: BarsProps) {
  const peak = Math.max(1, ...buckets.map((b) => b.count));
  const total = buckets.reduce((s, b) => s + b.count, 0);
  const c = colors[series];
  const mine = picked?.series === series ? picked.ts : null;
  return (
    <div>
      <div className="mb-1 flex justify-between text-[13px] text-ink-soft">
        <span className="font-semibold capitalize text-ink">{series}</span>
        <span className="tabular-nums">{total.toLocaleString("en")} in the period</span>
      </div>
      <div className="flex h-12 items-end gap-[3px]" role="group" aria-label={`${series} per ${labels[step] ?? "interval"}, highest ${peak}`}>
        {buckets.map((b, i) => (
          <button key={b.ts} type="button" disabled={!b.count || !onPick} onClick={() => onPick?.(b, series)}
            title={`${time(b.ts, step)}: ${b.count} ${series}${b.count ? " (show them)" : ""}`}
            aria-label={`${time(b.ts, step)}: ${b.count} ${series}`} aria-pressed={mine === b.ts}
            className="min-w-1 flex-1 rounded-t-[2px] enabled:cursor-pointer enabled:hover:opacity-80"
            style={{ height: `${Math.max(b.count ? 4 : 1, Math.round((b.count / peak) * 100))}%`,
              opacity: picked && mine !== b.ts ? 0.35 : 1,
              background: mine === b.ts || (!picked && i === buckets.length - 1) ? c.strong : b.count ? c.soft : "#e3e8ed" }} />
        ))}
      </div>
    </div>
  );
}

type Props = { served: Bucket[]; refused: Bucket[]; step: number; picked?: Picked; onPick?: (b: Bucket, s: Series) => void };

/**
 * Served and refused requests per interval, on the same time axis. A bar is a
 * button: it shows the events of its interval and kind.
 */
export function ActivityChart({ served, refused, step, picked = null, onPick }: Props) {
  const ticks = [0, Math.floor(refused.length / 2), refused.length - 1];
  return (
    <section aria-label="Requests over time" className="flex flex-col gap-3 rounded-[var(--radius-panel)] border border-rule-soft bg-surface px-4 pb-2.5 pt-3.5">
      <span className="text-[13px] font-semibold text-ink">Requests per {labels[step] ?? "interval"}</span>
      <Bars series="served" buckets={served} step={step} picked={picked} onPick={onPick} />
      <Bars series="refused" buckets={refused} step={step} picked={picked} onPick={onPick} />
      {refused.length ? (
        <div className="-mt-2 flex justify-between text-xs tabular-nums text-ink-soft">
          {ticks.map((i) => <span key={i}>{time(refused[i]?.ts ?? "", step)}</span>)}
        </div>
      ) : null}
    </section>
  );
}
