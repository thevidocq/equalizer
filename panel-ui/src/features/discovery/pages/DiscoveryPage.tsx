import { useState } from "react";
import { useParams } from "react-router";
import { Notice, Table, formatDate } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { canManage, hasRole, useMe } from "@/features/auth";
import { useApp } from "@/features/sites";
import { useLLM } from "@/features/settings";
import { useDiscoveries, useDiscovery, type Discovery } from "../hooks/useDiscovery";
import { ScheduleForm } from "../components/ScheduleForm";
import { StartForm } from "../components/StartForm";
import { Report } from "../components/Report";

const statusText = { queued: "Queued", running: "Running", done: "Done", failed: "Failed" } as const;

/** Site → Discovery: explore the site, read the report, open the proposed draft. */
export function DiscoveryPage() {
  const { tenant = "", app = "" } = useParams();
  const me = useMe().data;
  const site = useApp(app).data;
  const list = useDiscoveries(app);
  const isAdmin = canManage(me, tenant);
  const llm = useLLM(tenant, isAdmin);
  const [picked, setPicked] = useState<number | null>(null);
  const latest = list.data?.[0]?.id ?? null;
  const shown = useDiscovery(app, picked ?? latest);
  if (!site) return null;
  const aiState = !isAdmin ? "unknown" : llm.data?.provider ? "on" : "off";

  return (
    <div className="flex flex-col gap-6">
      {hasRole(me, tenant, "admin") ? (
        <StartForm tenant={tenant} app={app} firstTime={site.published_version == null} aiState={aiState} isAdmin={isAdmin} />
      ) : null}
      {isAdmin ? <ScheduleForm key={`${site.discovery_schedule}-${site.discovery_time?.hour}-${site.discovery_time?.weekday}`} site={site} /> : null}
      {list.isError ? <Notice tone="error">{describeError(list.error)}</Notice> : null}
      {list.data?.length ? (
        <Table<Discovery> caption="Discoveries" rows={list.data} rowKey={(d) => String(d.id)} columns={[
          { header: "Started", cell: (d) => (
            <button type="button" className="font-semibold text-signal hover:underline" onClick={() => setPicked(d.id)}>{formatDate(d.created_at)}</button>) },
          { header: "By", cell: (d) => (d.trigger === "schedule" ? "Schedule" : d.requested_by ?? "") },
          { header: "AI", cell: (d) => (!d.use_ai ? "No" : (d.report as { ai?: { used?: boolean } } | null)?.ai?.used === false ? "Asked, not used" : "Yes") },
          { header: "Status", cell: (d) => statusText[d.status] },
          { header: "Draft", cell: (d) => (d.draft_version ? `Version ${d.draft_version}` : "") },
        ]} />
      ) : null}
      {shown.data ? <Report d={shown.data} canAdd={isAdmin} /> : null}
    </div>
  );
}
