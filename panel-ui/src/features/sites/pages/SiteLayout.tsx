import { useCallback, useState } from "react";
import { Outlet, useParams, useSearchParams } from "react-router";
import { Notice, PageHeader, TabNav } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { canManage, useMe } from "@/features/auth";
import { AskAIButton, AssistantPanel } from "@/features/assistant";
import { useApp } from "../hooks/useSites";
import { SiteStatus } from "../components/SiteStatus";
import { DeleteSite } from "../components/DeleteSite";

/** A protected site: header and tabs, the tab content in the outlet, the assistant on the side. */
export function SiteLayout() {
  const { tenant = "", app = "" } = useParams();
  const site = useApp(app);
  const me = useMe().data;
  const [params, setParams] = useSearchParams();
  const fromHistory = params.get("assistant"); // a conversation opened from History on another site
  const [asking, setAsking] = useState(!!fromHistory);
  const close = useCallback(() => {
    setAsking(false);
    setParams((p) => { p.delete("assistant"); return p; }, { replace: true });
  }, [setParams]);
  if (site.isError) return <Notice tone="error">{describeError(site.error)}</Notice>;
  if (!site.data) return null;
  const base = `/t/${tenant}/sites/${app}`;
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={site.data.name} subtitle={<span className="flex flex-wrap items-center gap-x-4 gap-y-1">
        <span>{site.data.public_domain}</span><SiteStatus app={site.data} withLabel /></span>}
        actions={<>
          <AskAIButton open={asking || !!fromHistory} onToggle={() => (asking || fromHistory ? close() : setAsking(true))} />
          {canManage(me, tenant) ? <DeleteSite tenant={tenant} app={app} name={site.data.name} /> : null}
        </>} />
      <TabNav label="Site sections" tabs={[
        { to: `${base}/discovery`, label: "Discovery" },
        { to: `${base}/maps`, label: "Map" },
        { to: `${base}/isolation`, label: "Isolation" },
        { to: `${base}/certificate`, label: "Certificate" },
        { to: `${base}/login`, label: "Login" },
      ]} />
      <Outlet />
      <AssistantPanel key={app} tenant={tenant} app={app} siteName={site.data.name} mapVersion={site.data.published_version ?? null}
        canDraft={canManage(me, tenant)} open={asking || !!fromHistory} initialConversation={fromHistory} onClose={close} />
    </div>
  );
}
