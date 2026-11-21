import { useEffect, useState, type FormEvent } from "react";
import { startOfDay, today } from "@/shared/ui";
import type { AuditFilter } from "../hooks/useAudit";

export type Range = "24h" | "7d" | "30d" | "90d" | "all" | "days";
export type Filters = { q: string; range: Range; fromDay: string; toDay: string };

const control = "min-h-9 rounded-[var(--radius-control)] border border-rule bg-surface px-2 text-sm text-ink";
const hours: Partial<Record<Range, number>> = { "24h": 24, "7d": 168, "30d": 720, "90d": 2160 };

/** The server query for the chosen filters; days are whole days in the person's time zone. */
export function toQuery(f: Filters, now = Date.now()): AuditFilter {
  const h = hours[f.range];
  if (h) return { q: f.q, from: new Date(now - h * 3_600_000).toISOString() };
  if (f.range !== "days") return { q: f.q };
  const next = (day: string) => new Date(Date.parse(`${day}T00:00:00Z`) + 86_400_000).toISOString().slice(0, 10);
  return {
    q: f.q,
    from: f.fromDay ? startOfDay(f.fromDay).toISOString() : undefined,
    to: f.toDay ? startOfDay(next(f.toDay)).toISOString() : undefined,
  };
}

export const initialFilters: Filters = { q: "", range: "7d", fromDay: "", toDay: "" };

/** Free search and the period, preset or from one day to another. */
export function AuditFilters({ value, onChange }: { value: Filters; onChange: (f: Filters) => void }) {
  const [q, setQ] = useState(value.q);
  useEffect(() => setQ(value.q), [value.q]);
  const search = (e: FormEvent) => { e.preventDefault(); onChange({ ...value, q: q.trim() }); };
  return (
    <section aria-label="Filters" className="flex flex-wrap items-end gap-3 rounded-[var(--radius-panel)] border border-rule-soft bg-surface px-4 py-3.5">
      <label className="flex flex-col gap-1 text-[13px] text-ink-soft">
        Period
        <select className={`${control} min-w-40`} value={value.range}
          onChange={(e) => onChange({ ...value, range: e.target.value as Range, toDay: value.toDay || today() })}>
          <option value="24h">Last 24 hours</option>
          <option value="7d">Last 7 days</option>
          <option value="30d">Last 30 days</option>
          <option value="90d">Last 90 days</option>
          <option value="all">All time</option>
          <option value="days">Choose the days…</option>
        </select>
      </label>
      {value.range === "days" ? (
        <>
          <label className="flex flex-col gap-1 text-[13px] text-ink-soft">
            From
            <input type="date" className={control} value={value.fromDay} max={value.toDay || undefined}
              onChange={(e) => onChange({ ...value, fromDay: e.target.value })} />
          </label>
          <label className="flex flex-col gap-1 text-[13px] text-ink-soft">
            To
            <input type="date" className={control} value={value.toDay} min={value.fromDay || undefined}
              onChange={(e) => onChange({ ...value, toDay: e.target.value })} />
          </label>
        </>
      ) : null}
      <form onSubmit={search} role="search" className="flex min-w-60 flex-1 flex-col gap-1 text-[13px] text-ink-soft">
        <label htmlFor="audit-search">Search</label>
        <input id="audit-search" type="search" value={q} maxLength={200} onChange={(e) => setQ(e.target.value)}
          onBlur={() => q.trim() !== value.q && onChange({ ...value, q: q.trim() })}
          placeholder="Person, site, connector or action" className={`${control} px-2.5`} />
      </form>
    </section>
  );
}
