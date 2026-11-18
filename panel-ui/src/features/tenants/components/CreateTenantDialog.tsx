import { useState, type FormEvent } from "react";
import { Button, Dialog, Notice, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useCreateTenant } from "../hooks/useTenants";
import { slugify } from "./slug";

export function CreateTenantDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const create = useCreateTenant();
  const toast = useToast();
  const [name, setName] = useState("");
  const [id, setId] = useState("");
  const [idTouched, setIdTouched] = useState(false);

  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate(
      { id: idTouched ? id : slugify(name), name },
      { onSuccess: (t) => { toast(`Tenant ${t.name} added`); onOpenChange(false); setName(""); setId(""); setIdTouched(false); } },
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title="Add a tenant" description="A tenant is a customer: its people see only its own sites.">
      <form onSubmit={submit} className="flex flex-col gap-4">
        {create.isError ? <Notice tone="error">{describeError(create.error)}</Notice> : null}
        <TextField label="Name" required maxLength={200} value={name} onChange={(e) => setName(e.target.value)} />
        <TextField label="Id" hint="Lower case letters, digits and hyphens. It appears in URLs and cannot change."
          required pattern="[a-z0-9][a-z0-9\-]{0,62}" value={idTouched ? id : slugify(name)}
          onChange={(e) => { setIdTouched(true); setId(e.target.value); }} />
        <div className="flex justify-end gap-2">
          <Button onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button type="submit" variant="primary" busy={create.isPending}>Add tenant</Button>
        </div>
      </form>
    </Dialog>
  );
}
