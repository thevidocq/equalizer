import { useParams } from "react-router";
import { EmptyState, Notice, PageHeader, formatDate } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useAudit } from "../hooks/useAudit";
import { describeEntry } from "../components/describe";

export function AuditPage() {
  const tenant = useParams().tenant;
  const audit = useAudit(tenant);
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="Activity" subtitle="Who changed what, newest first. The last 200 actions." />
      {audit.isError ? <Notice tone="error">{describeError(audit.error)}</Notice> : null}
      {audit.data?.length === 0 ? <EmptyState title="Nothing has happened yet" /> : null}
      <ol className="flex flex-col">
        {audit.data?.map((e) => (
          <li key={e.id} className="grid grid-cols-[11rem_1fr] gap-4 border-b border-rule-soft py-2.5 text-sm">
            <time dateTime={e.ts} className="text-ink-soft">{formatDate(e.ts)}</time>
            <span className={e.action === "auth.login_failed" || e.action === "auth.mfa_failed" ? "text-state-rejected" : ""}>{describeEntry(e)}</span>
          </li>
        ))}
      </ol>
    </div>
  );
}
