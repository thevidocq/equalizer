import { formatTick } from "@/shared/ui";

export type Bucket = { ts: string; count: number };

const labels: Record<number, string> = { 300: "5 minutes", 3600: "hour", 21600: "6 hours", 86400: "day" };
const RED = "#b42318";
const RED_SOFT = "#e4a39c";
const GREEN = "#2b7a4b";

function time(ts: string, step: number) {
  return ts ? formatTick(new Date(ts), step >= 86400) : "";
}

const pct = (n: number, peak: number) => `${n ? Math.max(4, Math.round((n / peak) * 100)) : 0}%`;

type Props = {
  refused: Bucket[];
  /** Served requests on the same intervals; null: Served is not selected. */
  served: Bucket[] | null;
  step: number;
  picked?: string | null;
  onPick?: (b: Bucket) => void;
};

/**
 * One chart of requests per interval: refused in red and, when Served is
 * selected, served behind them in faded green, on the same scale. A bar is
 * a button: it shows the events of its interval, for the kinds selected.
 */
export function ActivityChart({ refused, served, step, picked = null, onPick }: Props) {
  const bars = refused.map((b, i) => ({ ts: b.ts, refused: b.count, served: served?.[i]?.count ?? 0 }));
  const peak = Math.max(1, ...bars.map((b) => Math.max(b.refused, b.served)));
  const sum = (xs: Bucket[]) => xs.reduce((s, b) => s + b.count, 0);
  const ticks = [0, Math.floor(bars.length / 2), bars.length - 1];
  const per = labels[step] ?? "interval";
  return (
    <section aria-label="Requests over time" className="rounded-[var(--radius-panel)] border border-rule-soft bg-surface px-4 pb-2.5 pt-3.5">
      <div className="mb-2 flex flex-wrap justify-between gap-2 text-[13px] text-ink-soft">
        <span className="font-semibold text-ink">Requests per {per}</span>
        <span className="flex gap-4 tabular-nums">
          <span><span aria-hidden="true" className="mr-1.5 inline-block size-2 rounded-full" style={{ background: RED }} />{sum(refused).toLocaleString("en")} refused</span>
          {served ? (
            <span><span aria-hidden="true" className="mr-1.5 inline-block size-2 rounded-full opacity-40" style={{ background: GREEN }} />{sum(served).toLocaleString("en")} served</span>
          ) : null}
        </span>
      </div>
      <div className="flex h-16 items-end gap-[3px]" role="group" aria-label={`Requests per ${per}, highest ${peak}`}>
        {bars.map((b, i) => {
          const what = `${b.refused} refused${served ? `, ${b.served} served` : ""}`;
          const empty = !b.refused && !b.served;
          return (
            <button key={b.ts} type="button" disabled={empty || !onPick} onClick={() => onPick?.(refused[i] as Bucket)}
              title={`${time(b.ts, step)}: ${what}${empty ? "" : " (show them)"}`} aria-label={`${time(b.ts, step)}: ${what}`}
              aria-pressed={picked === b.ts}
              className="relative h-full min-w-1 flex-1 enabled:cursor-pointer enabled:hover:opacity-80"
              style={{ opacity: picked && picked !== b.ts ? 0.35 : 1 }}>
              {empty ? <span className="absolute inset-x-0 bottom-0 h-px" style={{ background: "#e3e8ed" }} /> : null}
              {b.served ? <span className="absolute inset-x-0 bottom-0 rounded-t-[2px] opacity-30" style={{ height: pct(b.served, peak), background: GREEN }} /> : null}
              {b.refused ? (
                <span className="absolute inset-x-0 bottom-0 rounded-t-[2px]" style={{ height: pct(b.refused, peak),
                  background: picked === b.ts || (!picked && i === bars.length - 1) ? RED : RED_SOFT }} />
              ) : null}
            </button>
          );
        })}
      </div>
      {bars.length ? (
        <div className="mt-1 flex justify-between text-xs tabular-nums text-ink-soft">
          {ticks.map((i) => <span key={i}>{time(bars[i]?.ts ?? "", step)}</span>)}
        </div>
      ) : null}
    </section>
  );
}
