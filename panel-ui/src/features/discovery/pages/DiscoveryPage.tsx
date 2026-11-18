import { useState } from "react";
import { useParams } from "react-router";
import { Notice, SelectField, Table, formatDate } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { canManage, hasRole, useMe } from "@/features/auth";
import { useApp } from "@/features/sites";
import { useLLM } from "@/features/settings";
import { useDiscoveries, useDiscovery, useSchedule, type Discovery } from "../hooks/useDiscovery";
import { StartForm } from "../components/StartForm";
import { Report } from "../components/Report";

const statusText = { queued: "Queued", running: "Running", done: "Done", failed: "Failed" } as const;

/** Site → Discovery: explore the site, read the report, open the proposed draft. */
export function DiscoveryPage() {
  const { tenant = "", app = "" } = useParams();
  const me = useMe().data;
  const site = useApp(app).data;
  const list = useDiscoveries(app);
  const schedule = useSchedule(app);
  const isAdmin = canManage(me, tenant);
  const llm = useLLM(tenant, isAdmin);
  const [picked, setPicked] = useState<number | null>(null);
  const latest = list.data?.[0]?.id ?? null;
  const shown = useDiscovery(app, picked ?? latest);
  if (!site) return null;
  const aiState = !isAdmin ? "unknown" : llm.data?.provider ? "on" : "off";

  return (
    <div className="flex flex-col gap-6">
      {hasRole(me, tenant, "admin", "editor") ? (
        <StartForm tenant={tenant} app={app} firstTime={site.published_version == null} aiState={aiState} isAdmin={isAdmin} />
      ) : null}
      {isAdmin ? (
        <div className="flex max-w-md flex-col gap-1">
          <SelectField label="Continuous discovery" value={site.discovery_schedule ?? "off"} disabled={site.published_version == null}
            onChange={(e) => schedule.mutate(e.target.value as "off" | "daily" | "weekly")}
            hint={site.published_version == null ? "Available once a map is published." : "Explores again and proposes a draft only when something changed."}>
            <option value="off">Off</option><option value="daily">Every day</option><option value="weekly">Every week</option>
          </SelectField>
          {schedule.isError ? <Notice tone="error">{describeError(schedule.error)}</Notice> : null}
        </div>
      ) : null}
      {list.isError ? <Notice tone="error">{describeError(list.error)}</Notice> : null}
      {list.data?.length ? (
        <Table<Discovery> caption="Discoveries" rows={list.data} rowKey={(d) => String(d.id)} columns={[
          { header: "Started", cell: (d) => (
            <button type="button" className="font-semibold text-signal hover:underline" onClick={() => setPicked(d.id)}>{formatDate(d.created_at)}</button>) },
          { header: "By", cell: (d) => (d.trigger === "schedule" ? "Schedule" : d.requested_by ?? "") },
          { header: "AI", cell: (d) => (d.use_ai ? "Yes" : "No") },
          { header: "Status", cell: (d) => statusText[d.status] },
          { header: "Draft", cell: (d) => (d.draft_version ? `Version ${d.draft_version}` : "") },
        ]} />
      ) : null}
      {shown.data ? <Report d={shown.data} /> : null}
    </div>
  );
}
