import { formatTick } from "@/shared/ui";
import { chartStyles, type ChartStyle, type Layer } from "./chartData";
import { ChartColumn } from "./ChartColumn";

const labels: Record<number, string> = { 300: "5 minutes", 3600: "hour", 21600: "6 hours", 86400: "day" };
const RED = "#b42318";
const GREEN = "#2b7a4b";

function time(ts: string, step: number) {
  return ts ? formatTick(new Date(ts), step >= 86400) : "";
}

type Props = {
  slots: string[];
  refused: Layer[];
  /** Served requests on the same intervals; null: Served is not selected. */
  served: number[] | null;
  step: number;
  style: ChartStyle;
  onStyle: (s: ChartStyle) => void;
  picked?: string | null;
  onPick?: (ts: string) => void;
};

/**
 * One chart of requests per interval, drawn as the viewer chooses: refused
 * over the served (overlaid), next to them (side by side, each on its own
 * scale) or a stack of the kinds selected. Served shows only when selected.
 * A bar is a button: it shows the events of its interval, for the kinds selected.
 */
export function ActivityChart({ slots, refused, served, step, style, onStyle, picked = null, onPick }: Props) {
  const total = slots.map((_, i) => refused.reduce((s, l) => s + (l.counts[i] ?? 0), 0));
  const sum = (xs: number[]) => xs.reduce((s, n) => s + n, 0);
  const peaks = {
    refused: Math.max(1, ...total),
    served: Math.max(1, ...(served ?? [])),
    both: Math.max(1, ...total.map((n, i) => Math.max(n, served?.[i] ?? 0))),
    stack: Math.max(1, ...total.map((n, i) => n + (served?.[i] ?? 0))),
  };
  const ticks = [0, Math.floor(slots.length / 2), slots.length - 1];
  const per = labels[step] ?? "interval";
  return (
    <section aria-label="Requests over time" className="rounded-[var(--radius-panel)] border border-rule-soft bg-surface px-4 pb-2.5 pt-3.5">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2 text-[13px] text-ink-soft">
        <span className="font-semibold text-ink">Requests per {per}</span>
        <span className="flex flex-wrap items-center gap-4 tabular-nums">
          <span><span aria-hidden="true" className="mr-1.5 inline-block size-2 rounded-full" style={{ background: RED }} />{sum(total).toLocaleString("en")} refused</span>
          {served ? (
            <span><span aria-hidden="true" className="mr-1.5 inline-block size-2 rounded-full opacity-40" style={{ background: GREEN }} />{sum(served).toLocaleString("en")} served</span>
          ) : null}
          <span role="radiogroup" aria-label="Chart style" className="inline-flex overflow-hidden rounded-[var(--radius-control)] border border-rule">
            {chartStyles.map((s) => (
              <button key={s.id} type="button" role="radio" aria-checked={style === s.id} onClick={() => onStyle(s.id)}
                className={`px-2 py-0.5 text-xs ${style === s.id ? "bg-paper font-semibold text-ink" : "text-ink-soft hover:text-ink"}`}>
                {s.label}
              </button>
            ))}
          </span>
        </span>
      </div>
      <div className="flex h-16 items-end gap-[3px]" role="group" aria-label={`Requests per ${per}`}>
        {slots.map((ts, i) => {
          const what = `${total[i]} refused${served ? `, ${served[i] ?? 0} served` : ""}`;
          const empty = !total[i] && !served?.[i];
          return (
            <button key={ts} type="button" disabled={empty || !onPick} onClick={() => onPick?.(ts)}
              title={`${time(ts, step)}: ${what}${empty ? "" : " (show them)"}`} aria-label={`${time(ts, step)}: ${what}`}
              aria-pressed={picked === ts}
              className="relative h-full min-w-1 flex-1 enabled:cursor-pointer enabled:hover:opacity-80"
              style={{ opacity: picked && picked !== ts ? 0.35 : 1 }}>
              <ChartColumn style={style} i={i} refused={refused} total={total[i] ?? 0} served={served?.[i] ?? null}
                peaks={peaks} strong={picked === ts || (!picked && i === slots.length - 1)} />
            </button>
          );
        })}
      </div>
      {slots.length ? (
        <div className="mt-1 flex justify-between text-xs tabular-nums text-ink-soft">
          {ticks.map((i) => <span key={i}>{time(slots[i] ?? "", step)}</span>)}
        </div>
      ) : null}
    </section>
  );
}
