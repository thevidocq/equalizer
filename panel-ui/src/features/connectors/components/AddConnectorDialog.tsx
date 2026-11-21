import { useState, type FormEvent } from "react";
import { Button, Dialog, Notice, TextField } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import type { ConnectorCreated } from "@/shared/api/client";
import { useConnectorActions, useConnectorSetup } from "../hooks/useConnectors";
import { TokenReveal } from "./TokenReveal";

type Props = { tenant: string; open: boolean; onOpenChange: (o: boolean) => void };

export function AddConnectorDialog({ tenant, open, onOpenChange }: Props) {
  const { create } = useConnectorActions(tenant);
  const platform = useConnectorSetup(tenant).data?.version;
  const relayUrl = useConnectorSetup(tenant).data?.relayUrl;
  const [id, setId] = useState(`cn-${tenant}-01`.slice(0, 63));
  const [created, setCreated] = useState<ConnectorCreated | null>(null);

  function close(o: boolean) {
    onOpenChange(o);
    if (!o) {
      setCreated(null);
      create.reset();
    }
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate(id, { onSuccess: setCreated });
  }

  return (
    <Dialog open={open} onOpenChange={close} title={created ? `Connector ${created.id} added` : "Add a connector"}
      description={created ? undefined : "Install one per server: it opens the tunnel from the inside (no inbound port) and serves every site of that server."}>
      {created ? (
        <TokenReveal connectorId={created.id} token={created.token} version={platform} relayUrl={relayUrl} />
      ) : (
        <form onSubmit={submit} className="flex flex-col gap-4">
          {create.isError ? <Notice tone="error">{describeError(create.error)}</Notice> : null}
          <TextField label="Connector id" required pattern="[a-z0-9][a-z0-9_\-]{1,62}" value={id} onChange={(e) => setId(e.target.value)}
            hint="Unique in the installation, for example the server's name. Maps use it as connector_id." />
          <div className="flex justify-end gap-2">
            <Button onClick={() => close(false)}>Cancel</Button>
            <Button type="submit" variant="primary" busy={create.isPending}>Add connector</Button>
          </div>
        </form>
      )}
    </Dialog>
  );
}
