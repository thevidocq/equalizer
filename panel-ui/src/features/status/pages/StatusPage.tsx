import { useParams } from "react-router";
import { describeError } from "@/shared/api/errors";
import { EmptyState, Notice, PageHeader, formatDate } from "@/shared/ui";
import { useTenantStatus } from "../hooks/useStatus";
import { SiteRow } from "../components/SiteRow";

/** Is each site working, and if not, which part is broken. */
export function StatusPage() {
  const tenant = useParams().tenant ?? "";
  const st = useTenantStatus(tenant);
  const down = st.data?.sites.filter((s) => s.state === "down").length ?? 0;
  return (
    <div className="flex flex-col gap-5">
      <PageHeader title="Status"
        subtitle={st.data ? `Checked ${formatDate(st.data.checked_at)}. Refreshed every 30 seconds.` : "Whether each site works, and which part is broken."} />
      {st.isError ? <Notice tone="error">{describeError(st.error)}</Notice> : null}
      {st.data && !st.data.logs_available ? (
        <Notice tone="info">The log store does not answer: traffic and the last 7 days are not shown.</Notice>
      ) : null}
      {down ? <Notice tone="error">{down === 1 ? "1 site is down." : `${down} sites are down.`} The row says which part to look at.</Notice> : null}
      {st.data?.sites.length === 0 ? <EmptyState title="No sites yet">Protect a site and it appears here.</EmptyState> : null}
      {st.data?.sites.length ? (
        <>
          <ul className="flex flex-col border-t border-rule-soft">
            {st.data.sites.map((s) => <SiteRow key={s.app} site={s} />)}
          </ul>
          <p className="flex flex-wrap gap-4 text-xs text-ink-soft">
            <span><span aria-hidden="true" className="mr-1 inline-block size-2.5 rounded-[2px] bg-[#b7d7c2]" />No problems</span>
            <span><span aria-hidden="true" className="mr-1 inline-block size-2.5 rounded-[2px] bg-[#d9a441]" />Legacy errors</span>
            <span><span aria-hidden="true" className="mr-1 inline-block size-2.5 rounded-[2px] bg-[#b42318]" />Connector dropped</span>
          </p>
        </>
      ) : null}
    </div>
  );
}
