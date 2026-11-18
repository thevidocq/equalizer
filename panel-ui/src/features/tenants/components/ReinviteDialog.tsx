import { useState, type FormEvent } from "react";
import { Button, Dialog, Notice, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useReinvite } from "../hooks/useTenants";

type Props = { tenant: { id: string; name: string } | null; onClose: () => void };

/** Replaces the first administrator's invitation; the previous link stops working. */
export function ReinviteDialog({ tenant, onClose }: Props) {
  const reinvite = useReinvite(tenant?.id ?? "");
  const toast = useToast();
  const [form, setForm] = useState({ admin_name: "", admin_email: "" });

  function submit(e: FormEvent) {
    e.preventDefault();
    reinvite.mutate(form, {
      onSuccess: (inv) => {
        toast(inv.sent ? `Invitation sent to ${inv.email}` : "The invitation email could not be sent. Check the mail settings.");
        setForm({ admin_name: "", admin_email: "" }); onClose();
      },
    });
  }

  return (
    <Dialog open={tenant != null} onOpenChange={(o) => { if (!o) onClose(); }} title={`Invite the administrator of ${tenant?.name ?? ""} again`}
      description="Use this when the email did not arrive or the address was wrong. The previous link stops working.">
      <form onSubmit={submit} className="flex flex-col gap-4">
        {reinvite.isError ? <Notice tone="error">{describeError(reinvite.error)}</Notice> : null}
        <TextField label="Administrator's name" required value={form.admin_name}
          onChange={(e) => setForm({ ...form, admin_name: e.target.value })} />
        <TextField label="Administrator's email" type="email" required value={form.admin_email}
          onChange={(e) => setForm({ ...form, admin_email: e.target.value })} />
        <div className="flex justify-end gap-2">
          <Button onClick={onClose}>Cancel</Button>
          <Button type="submit" variant="primary" busy={reinvite.isPending}>Send invitation</Button>
        </div>
      </form>
    </Dialog>
  );
}
