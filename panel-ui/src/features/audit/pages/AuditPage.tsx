import { useMemo, useState } from "react";
import { Button, EmptyState, Notice, PageHeader, Spinner, formatDate, timeZone } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useAudit } from "../hooks/useAudit";
import { describeEntry, isWarning } from "../components/describe";
import { AuditFilters, initialFilters, toQuery, type Filters } from "../components/AuditFilters";

export function AuditPage() {
  const [filters, setFilters] = useState<Filters>(initialFilters);
  const query = useMemo(() => toQuery(filters), [filters]);
  const audit = useAudit(query);
  const entries = audit.data?.pages.flatMap((p) => p.items) ?? [];
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="Activity" subtitle={`Who did what, newest first. Times in ${timeZone()} (change it in Account).`} />
      <AuditFilters value={filters} onChange={setFilters} />
      {audit.isError ? <Notice tone="error">{describeError(audit.error)}</Notice> : null}
      {audit.isPending ? <Spinner label="Loading the activity" /> : null}
      {audit.isSuccess && entries.length === 0 ? (
        <EmptyState title={filters.q || filters.range !== "all" ? "Nothing matches these filters" : "Nothing has happened yet"} />
      ) : null}
      <ol className="flex flex-col">
        {entries.map((e) => (
          <li key={e.id} className="grid gap-x-4 gap-y-0.5 border-b border-rule-soft py-2.5 text-sm sm:grid-cols-[15rem_1fr]">
            <time dateTime={e.ts} className="tabular-nums text-ink-soft">{formatDate(e.ts)}</time>
            <span className={isWarning(e) ? "text-state-rejected" : ""}>{describeEntry(e)}</span>
          </li>
        ))}
      </ol>
      {audit.hasNextPage ? (
        <Button variant="secondary" className="self-start" disabled={audit.isFetchingNextPage} onClick={() => void audit.fetchNextPage()}>
          {audit.isFetchingNextPage ? "Loading…" : "Show older"}
        </Button>
      ) : null}
    </div>
  );
}
