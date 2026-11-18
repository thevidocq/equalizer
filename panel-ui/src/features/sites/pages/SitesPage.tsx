import { useState } from "react";
import { Link, useParams } from "react-router";
import { Button, EmptyState, Notice, PageHeader, Table } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import type { App } from "@/shared/api/client";
import { canManage, useMe } from "@/features/auth";
import { useApps } from "../hooks/useSites";
import { CreateSiteDialog } from "../components/CreateSiteDialog";
import { SiteStatus } from "../components/SiteStatus";

export function SitesPage() {
  const tenant = useParams().tenant ?? "";
  const apps = useApps(tenant);
  const me = useMe();
  const [adding, setAdding] = useState(false);
  const manage = canManage(me.data, tenant);
  const add = manage ? <Button variant="primary" onClick={() => setAdding(true)}>Protect a site</Button> : null;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="Protected sites" actions={add} />
      {apps.isError ? <Notice tone="error">{describeError(apps.error)}</Notice> : null}
      {apps.data?.length === 0 ? (
        <EmptyState title="No protected sites yet" action={add}>
          Add a site, install its connector next to the legacy server, then publish its map.
        </EmptyState>
      ) : null}
      {apps.data && apps.data.length > 0 ? (
        <Table<App> caption="Protected sites" rows={apps.data} rowKey={(a) => a.id} columns={[
          { header: "Site", cell: (a) => <Link className="font-semibold text-signal hover:underline" to={`/t/${tenant}/sites/${a.id}`}>{a.name}</Link> },
          { header: "Public domain", cell: (a) => a.public_domain },
          { header: "Status", cell: (a) => <SiteStatus app={a} withLabel /> },
        ]} />
      ) : null}
      <CreateSiteDialog tenant={tenant} open={adding} onOpenChange={setAdding} />
    </div>
  );
}
