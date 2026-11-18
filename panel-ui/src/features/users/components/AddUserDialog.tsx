import { useState, type FormEvent } from "react";
import { Button, Dialog, Notice, SelectField, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useCreateUser, type Role } from "../hooks/useUsers";
import { roleText } from "./roles";

type Props = { tenant: string; open: boolean; onOpenChange: (o: boolean) => void };

export function AddUserDialog({ tenant, open, onOpenChange }: Props) {
  const create = useCreateUser(tenant);
  const toast = useToast();
  const [form, setForm] = useState({ email: "", name: "", password: "", role: "editor" as Role });
  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => setForm({ ...form, [k]: e.target.value });

  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate(form, {
      onSuccess: (u) => { toast(`${u.name} added`); onOpenChange(false); setForm({ email: "", name: "", password: "", role: "editor" }); },
    });
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title="Add a person" description="Give them the password in person or over a separate channel.">
      <form onSubmit={submit} className="flex flex-col gap-4">
        {create.isError ? <Notice tone="error">{describeError(create.error)}</Notice> : null}
        <TextField label="Name" required value={form.name} onChange={set("name")} />
        <TextField label="Email" type="email" required value={form.email} onChange={set("email")} />
        <TextField label="First password" type="password" autoComplete="new-password" required minLength={12}
          value={form.password} onChange={set("password")} hint="At least 12 characters." />
        <SelectField label="Role" value={form.role} onChange={set("role")} hint={roleText[form.role].does}>
          {(Object.keys(roleText) as Role[]).map((r) => <option key={r} value={r}>{roleText[r].name}</option>)}
        </SelectField>
        <div className="flex justify-end gap-2">
          <Button onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button type="submit" variant="primary" busy={create.isPending}>Add person</Button>
        </div>
      </form>
    </Dialog>
  );
}
