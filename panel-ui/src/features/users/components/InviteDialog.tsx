import { useState, type FormEvent } from "react";
import { Button, Dialog, Notice, SelectField, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import type { Role } from "../hooks/useUsers";
import { useInvite } from "../hooks/useInvitations";
import { roleText } from "./roles";

type Props = { tenant: string; open: boolean; onOpenChange: (o: boolean) => void };
const empty = { email: "", name: "", role: "editor" as Role };

export function InviteDialog({ tenant, open, onOpenChange }: Props) {
  const invite = useInvite(tenant);
  const toast = useToast();
  const [form, setForm] = useState(empty);
  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => setForm({ ...form, [k]: e.target.value });

  function submit(e: FormEvent) {
    e.preventDefault();
    invite.mutate(form, {
      onSuccess: (inv) => {
        toast(inv.sent ? `Invitation sent to ${inv.email}` : `The email to ${inv.email} could not be sent. Revoke and try again later.`);
        onOpenChange(false); setForm(empty);
      },
    });
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title="Invite a person"
      description="They get an email with a link to choose their own password. The link works once and expires in 48 hours.">
      <form onSubmit={submit} className="flex flex-col gap-4">
        {invite.isError ? <Notice tone="error">{describeError(invite.error)}</Notice> : null}
        <TextField label="Name" required value={form.name} onChange={set("name")} />
        <TextField label="Email" type="email" required value={form.email} onChange={set("email")} />
        <SelectField label="Role" value={form.role} onChange={set("role")} hint={roleText[form.role].does}>
          {(Object.keys(roleText) as Role[]).map((r) => <option key={r} value={r}>{roleText[r].name}</option>)}
        </SelectField>
        <div className="flex justify-end gap-2">
          <Button onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button type="submit" variant="primary" busy={invite.isPending}>Send invitation</Button>
        </div>
      </form>
    </Dialog>
  );
}
