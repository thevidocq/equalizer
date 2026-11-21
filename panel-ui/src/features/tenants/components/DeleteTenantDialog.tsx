import { useState, type FormEvent } from "react";
import { Button, Dialog, Notice, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useDeleteTenant } from "../hooks/useTenants";

type Props = { tenant: { id: string; name: string } | null; onClose: () => void };

/** Deleting a customer cannot be undone: its id is typed again to confirm. */
export function DeleteTenantDialog({ tenant, onClose }: Props) {
  const remove = useDeleteTenant();
  const toast = useToast();
  const [typed, setTyped] = useState("");

  function submit(e: FormEvent) {
    e.preventDefault();
    if (!tenant) return;
    remove.mutate(tenant.id, { onSuccess: () => { toast(`${tenant.name} deleted`); setTyped(""); onClose(); } });
  }

  return (
    <Dialog open={tenant != null} onOpenChange={(o) => { if (!o) onClose(); }} title={`Delete ${tenant?.name ?? ""}`}
      description="Its sites stop being served, its connectors are disconnected, its people can no longer log in. This cannot be undone.">
      <form onSubmit={submit} className="flex flex-col gap-4">
        {remove.isError ? <Notice tone="error">{describeError(remove.error)}</Notice> : null}
        <p className="text-sm text-ink-soft">
          Deleted with it: sites and maps, connectors, people and invitations, settings and activity. Traffic logs expire on their own.
        </p>
        <TextField label={`Type ${tenant?.id ?? ""} to confirm`} autoComplete="off" value={typed} onChange={(e) => setTyped(e.target.value)} />
        <div className="flex justify-end gap-2">
          <Button onClick={onClose}>Cancel</Button>
          <Button type="submit" variant="danger" busy={remove.isPending} disabled={typed !== tenant?.id}>Delete customer</Button>
        </div>
      </form>
    </Dialog>
  );
}
