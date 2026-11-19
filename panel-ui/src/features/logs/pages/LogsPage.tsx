import { useMemo, useState } from "react";
import { useParams } from "react-router";
import { hasRole, useMe } from "@/features/auth";
import { useApps } from "@/features/sites";
import { describeError } from "@/shared/api/errors";
import { Button, EmptyState, Notice, PageHeader } from "@/shared/ui";
import { defaultKinds, eventKey, kinds } from "../components/kinds";
import { LogFilters } from "../components/LogFilters";
import { RefusedChart } from "../components/RefusedChart";
import { EventTable } from "../components/EventTable";
import { EventDetail } from "../components/EventDetail";
import { useLogExport, useLogStats, useLogs, useRestartLogs, type LogFilter } from "../hooks/useLogs";

/** What the gateway served and refused on the tenant's sites. */
export function LogsPage() {
  const tenant = useParams().tenant ?? "";
  const me = useMe().data;
  const apps = useApps(tenant).data ?? [];
  const [on, setOn] = useState<string[]>(defaultKinds);
  const [base, setBase] = useState<Omit<LogFilter, "types">>({ app: "", q: "", since: "24h" });
  const [live, setLive] = useState(true);
  const [selected, setSelected] = useState<string | null>(null);
  const filter: LogFilter = useMemo(
    () => ({ ...base, types: kinds.filter((k) => on.includes(k.id)).flatMap((k) => k.types) }), [base, on]);

  const logs = useLogs(tenant, filter, live);
  const stats = useLogStats(tenant, filter, live);
  const exporter = useLogExport(tenant);
  const restart = useRestartLogs(tenant);
  const events = logs.data?.pages.flatMap((p) => p.items) ?? [];
  // The detail follows the event, not its row: live refresh adds rows on top.
  const current = events.find((e) => eventKey(e) === selected) ?? events[0];
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
              <span aria-hidden="true" className="size-2 rounded-full" style={{ background: live ? "#2b7a4b" : "#6b7684" }} />
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
        <div className="flex flex-wrap items-start gap-5">
          <section aria-label="Events" className="min-w-0 flex-[999_1_560px] overflow-hidden rounded-[var(--radius-panel)] border border-rule-soft bg-surface">
            <EventTable events={events} apps={apps} selected={current ? eventKey(current) : null} onSelect={setSelected} />
            <div className="flex items-center justify-between border-t border-rule-soft px-4 py-2.5 text-[13px] text-ink-soft">
              <span>{events.length} events shown, newest first</span>
              {logs.hasNextPage ? (
                <Button busy={logs.isFetchingNextPage} onClick={() => { setLive(false); void logs.fetchNextPage(); }}>Load older (pauses Live)</Button>
              ) : null}
            </div>
          </section>
          {current ? (
            <EventDetail event={current} apps={apps} tenant={tenant} canEditMap={hasRole(me, tenant, "admin", "editor")}
              onOnlyIP={(ip) => change({ ...filter, q: ip })} />
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
