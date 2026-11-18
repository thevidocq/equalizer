import { useState } from "react";
import { useParams } from "react-router";
import { Button, Dialog, EmptyState, Notice, PageHeader, Table, formatDate, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import type { Connector, ConnectorCreated } from "@/shared/api/client";
import { canManage, useMe } from "@/features/auth";
import { useConnectorActions, useConnectors } from "../hooks/useConnectors";
import { AddConnectorDialog } from "../components/AddConnectorDialog";
import { TokenReveal } from "../components/TokenReveal";

export function ConnectorsPage() {
  const tenant = useParams().tenant ?? "";
  const list = useConnectors(tenant);
  const { rotate, remove } = useConnectorActions(tenant);
  const me = useMe();
  const toast = useToast();
  const manage = canManage(me.data, tenant);
  const [adding, setAdding] = useState(false);
  const [rotated, setRotated] = useState<ConnectorCreated | null>(null);
  const [removing, setRemoving] = useState<string | null>(null);
  const failure = list.error ?? rotate.error ?? remove.error;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="Connectors"
        subtitle="One connector per server: it serves every site on the addresses it may reach. Rotating a token closes the tunnels that use the old one within 15 seconds."
        actions={manage ? <Button variant="primary" onClick={() => setAdding(true)}>Add a connector</Button> : null} />
      {failure ? <Notice tone="error">{describeError(failure)}</Notice> : null}
      {list.data?.length === 0 ? (
        <EmptyState title="No connectors yet">Add one and run the command it gives you on the legacy server, or on a machine next to it.</EmptyState>
      ) : null}
      {list.data && list.data.length > 0 ? (
        <Table<Connector> caption="Connectors" rows={list.data} rowKey={(c) => c.id} columns={[
          { header: "Id", cell: (c) => <span className="font-semibold">{c.id}</span> },
          { header: "Used by", cell: (c) => (c.used_by?.length ? c.used_by.join(", ") : <span className="text-ink-soft">No published map yet</span>) },
          { header: "Added", cell: (c) => formatDate(c.created_at) },
          { header: "Token changed", cell: (c) => formatDate(c.rotated_at) || "Never" },
          { header: "", className: "text-right", cell: (c) => manage ? (
            <span className="inline-flex gap-2">
              <Button onClick={() => rotate.mutate(c.id, { onSuccess: setRotated })}>Rotate token</Button>
              <Button variant="danger" disabled={!!c.used_by?.length} title={c.used_by?.length ? "A published map uses it" : undefined}
                onClick={() => setRemoving(c.id)}>Remove</Button>
            </span>) : null },
        ]} />
      ) : null}
      <AddConnectorDialog tenant={tenant} open={adding} onOpenChange={setAdding} />
      <Dialog open={!!rotated} onOpenChange={(o) => !o && setRotated(null)} title={`New token for ${rotated?.id ?? ""}`}>
        {rotated ? <TokenReveal connectorId={rotated.id} token={rotated.token} /> : null}
      </Dialog>
      <Dialog open={!!removing} onOpenChange={(o) => !o && setRemoving(null)} title={`Remove ${removing ?? ""}?`}
        description="Its tunnels close and the site stops answering through it.">
        <div className="flex justify-end gap-2">
          <Button onClick={() => setRemoving(null)}>Cancel</Button>
          <Button variant="danger" busy={remove.isPending} onClick={() => removing && remove.mutate(removing, {
            onSuccess: () => { toast(`${removing} removed`); setRemoving(null); } })}>Remove connector</Button>
        </div>
      </Dialog>
    </div>
  );
}
