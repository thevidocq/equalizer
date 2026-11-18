import { useState } from "react";
import { Link } from "react-router";
import { Button, EmptyState, Notice, PageHeader, Table, formatDate } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import type { Tenant } from "@/shared/api/client";
import { useMe } from "@/features/auth";
import { useTenants } from "../hooks/useTenants";
import { CreateTenantDialog } from "../components/CreateTenantDialog";

export function TenantsPage() {
  const tenants = useTenants();
  const me = useMe();
  const [adding, setAdding] = useState(false);
  const isAdmin = me.data?.platform_role === "admin";

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="Tenants" subtitle="Every customer of this installation."
        actions={isAdmin ? <Button variant="primary" onClick={() => setAdding(true)}>Add a tenant</Button> : null} />
      {tenants.isError ? <Notice tone="error">{describeError(tenants.error)}</Notice> : null}
      {tenants.data && tenants.data.length === 0 ? (
        <EmptyState title="No tenants yet" action={isAdmin ? <Button variant="primary" onClick={() => setAdding(true)}>Add a tenant</Button> : null}>
          Add the first customer, then protect its sites.
        </EmptyState>
      ) : null}
      {tenants.data && tenants.data.length > 0 ? (
        <Table<Tenant> caption="Tenants" rows={tenants.data} rowKey={(t) => t.id} columns={[
          { header: "Name", cell: (t) => <Link className="font-semibold text-signal hover:underline" to={`/t/${t.id}`}>{t.name}</Link> },
          { header: "Id", cell: (t) => t.id },
          { header: "Added", cell: (t) => formatDate(t.created_at) },
        ]} />
      ) : null}
      <CreateTenantDialog open={adding} onOpenChange={setAdding} />
    </div>
  );
}
