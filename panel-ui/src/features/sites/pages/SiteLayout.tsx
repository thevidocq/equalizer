import { Outlet, useParams } from "react-router";
import { Notice, PageHeader, TabNav } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { canManage, useMe } from "@/features/auth";
import { useApp } from "../hooks/useSites";
import { SiteStatus } from "../components/SiteStatus";
import { DeleteSite } from "../components/DeleteSite";

/** A protected site: header and tabs, the tab content in the outlet. */
export function SiteLayout() {
  const { tenant = "", app = "" } = useParams();
  const site = useApp(app);
  const me = useMe().data;
  if (site.isError) return <Notice tone="error">{describeError(site.error)}</Notice>;
  if (!site.data) return null;
  const base = `/t/${tenant}/sites/${app}`;
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={site.data.name} subtitle={<span className="flex flex-wrap items-center gap-x-4 gap-y-1">
        <span>{site.data.public_domain}</span><SiteStatus app={site.data} withLabel /></span>}
        actions={canManage(me, tenant) ? <DeleteSite tenant={tenant} app={app} name={site.data.name} /> : undefined} />
      <TabNav label="Site sections" tabs={[
        { to: `${base}/discovery`, label: "Discovery" },
        { to: `${base}/maps`, label: "Map" },
        { to: `${base}/isolation`, label: "Isolation" },
        { to: `${base}/certificate`, label: "Certificate" },
        { to: `${base}/login`, label: "Login" },
      ]} />
      <Outlet />
    </div>
  );
}
