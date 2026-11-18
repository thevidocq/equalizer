import { useState, type FormEvent } from "react";
import { Button, Dialog, Notice, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useCreateTenant } from "../hooks/useTenants";
import { slugify } from "./slug";

const empty = { name: "", id: "", admin_name: "", admin_email: "" };

export function CreateTenantDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const create = useCreateTenant();
  const toast = useToast();
  const [form, setForm] = useState(empty);
  const [idTouched, setIdTouched] = useState(false);
  const set = (k: keyof typeof empty) => (e: { target: { value: string } }) => setForm({ ...form, [k]: e.target.value });

  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate({ ...form, id: idTouched ? form.id : slugify(form.name) }, {
      onSuccess: (r) => {
        toast(r.invitation.sent
          ? `${r.tenant.name} added. Invitation sent to ${r.invitation.email}`
          : `${r.tenant.name} added, but the invitation email could not be sent. Check the mail settings.`);
        onOpenChange(false); setForm(empty); setIdTouched(false);
      },
    });
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title="Add a customer"
      description="The customer's administrator gets an email to choose a password. You will not see inside the tenant.">
      <form onSubmit={submit} className="flex flex-col gap-4">
        {create.isError ? <Notice tone="error">{describeError(create.error)}</Notice> : null}
        <TextField label="Customer name" required maxLength={200} value={form.name} onChange={set("name")} />
        <TextField label="Id" hint="Lower case letters, digits and hyphens. It appears in URLs and cannot change."
          required pattern="[a-z0-9][a-z0-9\-]{0,62}" value={idTouched ? form.id : slugify(form.name)}
          onChange={(e) => { setIdTouched(true); set("id")(e); }} />
        <TextField label="Administrator's name" required value={form.admin_name} onChange={set("admin_name")} />
        <TextField label="Administrator's email" type="email" required value={form.admin_email} onChange={set("admin_email")}
          hint="The invitation link works once and expires in 48 hours." />
        <div className="flex justify-end gap-2">
          <Button onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button type="submit" variant="primary" busy={create.isPending}>Add and invite</Button>
        </div>
      </form>
    </Dialog>
  );
}
