import { kinds, type Kind } from "./kinds";

export type Bucket = { ts: string; count: number };
/** How the chart draws served and refused requests; the viewer picks it. */
export type ChartStyle = "overlaid" | "side" | "stacked";
export const chartStyles: { id: ChartStyle; label: string }[] = [
  { id: "overlaid", label: "Overlaid" },
  { id: "side", label: "Side by side" },
  { id: "stacked", label: "Stacked" },
];

/** One kind of event on the chart: its counts per interval. */
export type Layer = { kind: Kind; counts: number[] };

/** What the chart shows for the kinds selected: a layer per refused kind,
 * served apart (null when Served is not selected), on the same intervals. */
export function chartLayers(byType: Record<string, Bucket[]>, slots: string[], on: string[]) {
  const counts = (k: Kind) => slots.map((_, i) => k.types.reduce((s, t) => s + (byType[t]?.[i]?.count ?? 0), 0));
  const picked = kinds.filter((k) => on.includes(k.id));
  const refused: Layer[] = picked.filter((k) => k.id !== "served").map((k) => ({ kind: k, counts: counts(k) }));
  const servedKind = picked.find((k) => k.id === "served");
  return { refused, served: servedKind ? counts(servedKind) : null };
}

const KEY = "equalizer.logs.chart";

/** The viewer's chart style, kept in this browser; overlaid by default. */
export function savedStyle(): ChartStyle {
  try {
    const v = window.localStorage.getItem(KEY);
    return chartStyles.some((s) => s.id === v) ? (v as ChartStyle) : "overlaid";
  } catch {
    return "overlaid";
  }
}

export function saveStyle(s: ChartStyle) {
  try {
    window.localStorage.setItem(KEY, s);
  } catch {
    // private browsing: the choice lasts the page
  }
}
