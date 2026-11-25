import type { ChartStyle, Layer } from "./chartData";

const RED = "#b42318";
const RED_SOFT = "#e4a39c";
const GREEN = "#2b7a4b";

const pct = (n: number, peak: number) => (n ? Math.max(4, Math.round((n / peak) * 100)) : 0);

type Peaks = { refused: number; served: number; both: number; stack: number };
type Props = { style: ChartStyle; i: number; refused: Layer[]; total: number; served: number | null; peaks: Peaks; strong: boolean };

function Bar({ h, color, faded, place }: { h: number; color: string; faded?: boolean; place: string }) {
  return h ? <span className={`absolute bottom-0 rounded-t-[2px] ${place} ${faded ? "opacity-30" : ""}`} style={{ height: `${h}%`, background: color }} /> : null;
}

/** The marks of one interval, in the chart style chosen. */
export function ChartColumn({ style, i, refused, total, served, peaks, strong }: Props) {
  const red = strong ? RED : RED_SOFT;
  if (!total && !served) return <span className="absolute inset-x-0 bottom-0 h-px bg-rule-soft" />;
  if (style === "side") {
    // each on its own scale: served requests outnumber refused ones
    return served === null ? (
      <Bar h={pct(total, peaks.refused)} color={red} place="inset-x-0" />
    ) : (
      <>
        <Bar h={pct(total, peaks.refused)} color={red} place="left-0 w-[48%]" />
        <Bar h={pct(served, peaks.served)} color={GREEN} faded place="right-0 w-[48%]" />
      </>
    );
  }
  if (style === "stacked") {
    // the kinds selected, served at the base, one scale
    const layers = [
      ...(served !== null ? [{ n: served, color: GREEN, opacity: 0.3 }] : []),
      ...refused.map((l) => ({ n: l.counts[i] ?? 0, color: l.kind.color, opacity: strong ? 1 : 0.6 })),
    ];
    let bottom = 0;
    return (
      <>
        {layers.map((l, j) => {
          if (!l.n) return null;
          const h = Math.max(2, (l.n / peaks.stack) * 100);
          const at = bottom;
          bottom += h;
          return <span key={j} className="absolute inset-x-0" style={{ bottom: `${at}%`, height: `${h}%`, background: l.color, opacity: l.opacity }} />;
        })}
      </>
    );
  }
  return (
    <>
      {served !== null ? <Bar h={pct(served, peaks.both)} color={GREEN} faded place="inset-x-0" /> : null}
      <Bar h={pct(total, peaks.both)} color={red} place="inset-x-0" />
    </>
  );
}
