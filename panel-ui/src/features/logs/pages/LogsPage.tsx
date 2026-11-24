import { useMemo, useState } from "react";
import { useParams, useSearchParams } from "react-router";
import { hasRole, useMe } from "@/features/auth";
import { useApps } from "@/features/sites";
import { describeError } from "@/shared/api/errors";
import { Button, EmptyState, Notice, PageHeader } from "@/shared/ui";
import { defaultKinds, kinds } from "../components/kinds";
import { LogFilters } from "../components/LogFilters";
import { RefusedChart } from "../components/RefusedChart";
import { EventTable } from "../components/EventTable";
import { EventDetail } from "../components/EventDetail";
import { useLogExport, useLogStats, useLogs, useRestartLogs, type LogFilter } from "../hooks/useLogs";

const periodOf = (v: string | null): LogFilter["since"] => (v === "1h" || v === "7d" || v === "30d" ? v : "24h");

/** What the gateway served and refused on the tenant's sites. */
export function LogsPage() {
  const tenant = useParams().tenant ?? "";
  const me = useMe().data;
  const apps = useApps(tenant).data ?? [];
  const [params] = useSearchParams(); // a link from elsewhere (the assistant) opens on its event
  const [on, setOn] = useState<string[]>(() => (params.get("q") ? kinds.map((k) => k.id) : defaultKinds));
  const [base, setBase] = useState<Omit<LogFilter, "types">>(() => ({
    app: params.get("app") ?? "", q: params.get("q") ?? "", since: periodOf(params.get("since")),
  }));
  const [live, setLive] = useState(true);
  const [selected, setSelected] = useState<string | null>(null);
  const filter: LogFilter = useMemo(
    () => ({ ...base, types: kinds.filter((k) => on.includes(k.id)).flatMap((k) => k.types) }), [base, on]);

  const logs = useLogs(tenant, filter, live);
  const stats = useLogStats(tenant, filter, live);
  const exporter = useLogExport(tenant);
  const restart = useRestartLogs(tenant);
  const events = logs.data?.pages.flatMap((p) => p.items) ?? [];
  const error = logs.error ?? stats.error ?? exporter.error;

  function change(f: LogFilter) {
    setBase({ app: f.app, q: f.q, since: f.since });
    setSelected(null);
  }
  function toggle(id: string) {
    setOn((o) => (o.includes(id) ? o.filter((x) => x !== id) : [...o, id]));
    setSelected(null);
  }

  return (
    <div className="flex flex-col gap-5">
      <PageHeader title="Logs" subtitle="What the gateway served and refused on your sites. Kept 30 days."
        actions={
          <>
            <Button aria-pressed={live} onClick={() => { if (!live) restart(); setLive(!live); }}>
              <span aria-hidden="true" className="relative flex size-2">
                {live ? <span className="absolute inline-flex size-full rounded-full bg-[#2b7a4b] opacity-75 motion-safe:animate-ping" /> : null}
                <span className="relative inline-flex size-2 rounded-full" style={{ background: live ? "#2b7a4b" : "#6b7684" }} />
              </span>
              {live ? "Live" : "Paused"}
            </Button>
            <Button busy={exporter.isPending} disabled={!on.length} onClick={() => exporter.mutate(filter)}>Export CSV</Button>
          </>
        } />
      {error ? <Notice tone="error">{describeError(error)}</Notice> : null}
      <LogFilters apps={apps} filter={filter} onChange={change} on={on} onToggle={toggle} counts={stats.data?.counts} />
      {stats.data ? <RefusedChart buckets={stats.data.refused} step={stats.data.step_seconds} /> : null}
      {!on.length ? <EmptyState title="Choose at least one kind of event" /> : null}
      {on.length && logs.data && !events.length ? (
        <EmptyState title="No events for these filters">Try a longer period or turn on more kinds of event.</EmptyState>
      ) : null}
      {events.length ? (
        <section aria-label="Events" className="overflow-hidden rounded-[var(--radius-panel)] border border-rule-soft bg-surface">
          {/* the open row is the event, not its place: live refresh adds rows on top */}
          <EventTable events={events} apps={apps} selected={selected} onSelect={(k) => setSelected((s) => (s === k ? null : k))}
            detail={(e) => (
              <EventDetail event={e} apps={apps} tenant={tenant} canEditMap={hasRole(me, tenant, "admin")}
                onOnlyIP={(ip) => change({ ...filter, q: ip })} />
            )} />
          <div className="flex items-center justify-between border-t border-rule-soft px-4 py-2.5 text-[13px] text-ink-soft">
            <span>{events.length} events shown, newest first</span>
            {logs.hasNextPage ? (
              <Button busy={logs.isFetchingNextPage} onClick={() => { setLive(false); void logs.fetchNextPage(); }}>Load older (pauses Live)</Button>
            ) : null}
          </div>
        </section>
      ) : null}
    </div>
  );
}
