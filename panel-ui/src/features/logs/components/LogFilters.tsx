import { useEffect, useState, type FormEvent } from "react";
import type { App } from "@/shared/api/client";
import { kinds, type Since } from "./kinds";
import type { LogFilter } from "../hooks/useLogs";

const select =
  "min-h-9 rounded-[var(--radius-control)] border border-rule bg-surface px-2 text-sm text-ink";

type Props = {
  apps: App[];
  filter: LogFilter;
  onChange: (f: LogFilter) => void;
  on: string[];
  onToggle: (kind: string) => void;
  counts: Record<string, number> | undefined;
};

/** Site, period, free search and the event chips with their counts. */
export function LogFilters({ apps, filter, onChange, on, onToggle, counts }: Props) {
  const [q, setQ] = useState(filter.q);
  useEffect(() => { setQ(filter.q); }, [filter.q]);
  function search(e: FormEvent) {
    e.preventDefault();
    onChange({ ...filter, q: q.trim() });
  }
  return (
    <section aria-label="Filters" className="flex flex-col gap-3 rounded-[var(--radius-panel)] border border-rule-soft bg-surface px-4 py-3.5">
      <div className="flex flex-wrap items-end gap-3">
        <label className="flex flex-col gap-1 text-[13px] text-ink-soft">
          Site
          <select className={`${select} min-w-44`} value={filter.app} onChange={(e) => onChange({ ...filter, app: e.target.value })}>
            <option value="">All sites</option>
            {apps.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
          </select>
        </label>
        <label className="flex flex-col gap-1 text-[13px] text-ink-soft">
          Period
          <select className={`${select} min-w-36`} value={filter.since} onChange={(e) => onChange({ ...filter, since: e.target.value as Since })}>
            <option value="1h">Last hour</option>
            <option value="24h">Last 24 hours</option>
            <option value="7d">Last 7 days</option>
            <option value="30d">Last 30 days</option>
          </select>
        </label>
        <form onSubmit={search} role="search" className="flex min-w-64 flex-1 flex-col gap-1 text-[13px] text-ink-soft">
          <label htmlFor="log-search">Search</label>
          <input id="log-search" type="search" value={q} maxLength={200} onChange={(e) => setQ(e.target.value)}
            onBlur={() => q.trim() !== filter.q && onChange({ ...filter, q: q.trim() })}
            placeholder="IP address, path or request ID"
            className="min-h-9 rounded-[var(--radius-control)] border border-rule bg-surface px-2.5 text-sm text-ink" />
        </form>
      </div>
      <div role="group" aria-label="Show" className="flex flex-wrap gap-2">
        {kinds.map((k) => {
          const active = on.includes(k.id);
          const n = counts ? k.types.reduce((s, t) => s + (counts[t] ?? 0), 0) : undefined;
          return (
            <button key={k.id} type="button" aria-pressed={active} onClick={() => onToggle(k.id)}
              className={`inline-flex min-h-9 items-center gap-1.5 rounded-full px-3 text-sm ${active
                ? "border border-ink bg-surface text-ink" : "border border-dashed border-rule bg-paper text-ink-soft"}`}>
              <span aria-hidden="true" className="size-2 rounded-full" style={{ background: k.color }} />
              {k.label}
              {n !== undefined ? <span className="tabular-nums text-ink-soft">{compact(n)}</span> : null}
            </button>
          );
        })}
      </div>
    </section>
  );
}

export function compact(n: number): string {
  return n >= 10_000 ? `${(n / 1000).toFixed(1).replace(/\.0$/, "")}k` : n.toLocaleString("en");
}
