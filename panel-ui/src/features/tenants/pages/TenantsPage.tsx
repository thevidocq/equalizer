import { useState } from "react";
import { Button, EmptyState, Notice, PageHeader, Table, formatDate } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import type { Schemas } from "@/shared/api/client";
import { useMe } from "@/features/auth";
import { useTenantOverview } from "../hooks/useTenants";
import { CreateTenantDialog } from "../components/CreateTenantDialog";
import { SiteDots } from "../components/SiteDots";
import { ReinviteDialog } from "../components/ReinviteDialog";

type Row = Schemas["TenantOverview"];

export function TenantsPage() {
  const tenants = useTenantOverview();
  const me = useMe();
  const [adding, setAdding] = useState(false);
  const [reinviting, setReinviting] = useState<Row | null>(null);
  const isAdmin = me.data?.platform_role === "admin";
  const add = isAdmin
    ? <Button variant="primary" onClick={() => setAdding(true)}>Add a customer</Button> : null;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="Tenants"
        subtitle="Every customer of this installation. Their maps, connectors, people and activity stay private to them."
        actions={add} />
      {tenants.isError ? <Notice tone="error">{describeError(tenants.error)}</Notice> : null}
      {tenants.data?.length === 0 ? (
        <EmptyState title="No customers yet">Add a customer: its first administrator gets an email to join.</EmptyState>
      ) : null}
      {tenants.data && tenants.data.length > 0 ? (
        <Table<Row> caption="Tenants" rows={tenants.data} rowKey={(t) => t.id} columns={[
          { header: "Name", cell: (t) => <span className="font-semibold">{t.name}</span> },
          { header: "Id", cell: (t) => <span className="font-mono text-sm">{t.id}</span> },
          { header: "Added", cell: (t) => formatDate(t.created_at) },
          { header: "Sites", className: "text-right", cell: (t) => t.site_count },
          { header: "Status", cell: (t) => (t.joined ? <SiteDots sites={t.sites} /> : (
            <span className="inline-flex flex-wrap items-center gap-3 text-state-review">
              Waiting for the administrator to join
              {isAdmin ? <Button onClick={() => setReinviting(t)}>Invite again</Button> : null}
            </span>
          )) },
        ]} />
      ) : null}
      <CreateTenantDialog open={adding} onOpenChange={setAdding} />
      <ReinviteDialog key={reinviting?.id ?? ""} tenant={reinviting} onClose={() => setReinviting(null)} />
    </div>
  );
}
