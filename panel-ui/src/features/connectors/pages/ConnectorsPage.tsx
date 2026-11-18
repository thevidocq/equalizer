import { useState } from "react";
import { useParams } from "react-router";
import { Button, Dialog, EmptyState, Notice, Table, formatDate, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import type { Connector, ConnectorCreated } from "@/shared/api/client";
import { canManage, useMe } from "@/features/auth";
import { useConnectorActions, useConnectors } from "../hooks/useConnectors";
import { AddConnectorDialog } from "../components/AddConnectorDialog";
import { TokenReveal } from "../components/TokenReveal";

export function ConnectorsPage() {
  const { tenant = "", app = "" } = useParams();
  const list = useConnectors(app);
  const { rotate, remove } = useConnectorActions(app);
  const me = useMe();
  const toast = useToast();
  const manage = canManage(me.data, tenant);
  const [adding, setAdding] = useState(false);
  const [rotated, setRotated] = useState<ConnectorCreated | null>(null);
  const [removing, setRemoving] = useState<string | null>(null);
  const failure = list.error ?? rotate.error ?? remove.error;

  return (
    <div className="flex flex-col gap-5">
      <div className="flex items-start justify-between gap-4">
        <p className="max-w-prose text-ink-soft">
          Connectors reach the legacy server for the gateway. Rotating a token closes the tunnels that use the old one within 15 seconds.
        </p>
        {manage ? <Button variant="primary" onClick={() => setAdding(true)}>Add a connector</Button> : null}
      </div>
      {failure ? <Notice tone="error">{describeError(failure)}</Notice> : null}
      {list.data?.length === 0 ? (
        <EmptyState title="No connectors yet">Add one and run the command it gives you on a machine next to the legacy server.</EmptyState>
      ) : null}
      {list.data && list.data.length > 0 ? (
        <Table<Connector> caption="Connectors" rows={list.data} rowKey={(c) => c.id} columns={[
          { header: "Id", cell: (c) => <span className="font-semibold">{c.id}</span> },
          { header: "Added", cell: (c) => formatDate(c.created_at) },
          { header: "Token changed", cell: (c) => formatDate(c.rotated_at) || "Never" },
          { header: "", className: "text-right", cell: (c) => manage ? (
            <span className="inline-flex gap-2">
              <Button onClick={() => rotate.mutate(c.id, { onSuccess: setRotated })}>Rotate token</Button>
              <Button variant="danger" onClick={() => setRemoving(c.id)}>Remove</Button>
            </span>) : null },
        ]} />
      ) : null}
      <AddConnectorDialog app={app} open={adding} onOpenChange={setAdding} />
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
