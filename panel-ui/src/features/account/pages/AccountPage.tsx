import { useState, type FormEvent } from "react";
import { Button, Notice, PageHeader, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useChangePassword, useMe } from "@/features/auth";
import { TimeZoneForm } from "../components/TimeZoneForm";

export function AccountPage() {
  const me = useMe().data;
  const change = useChangePassword();
  const toast = useToast();
  const [form, setForm] = useState({ current: "", next: "", again: "" });
  const mismatch = form.again !== "" && form.again !== form.next;

  function submit(e: FormEvent) {
    e.preventDefault();
    if (mismatch) return;
    change.mutate({ current_password: form.current, new_password: form.next }, {
      onSuccess: () => { toast("Password changed"); setForm({ current: "", next: "", again: "" }); },
    });
  }

  return (
    <div className="flex max-w-md flex-col gap-6">
      <PageHeader title="Your account" subtitle={me ? `${me.name}, ${me.email}` : undefined} />
      {me ? <TimeZoneForm key={me.time_zone ?? "UTC"} current={me.time_zone ?? "UTC"} /> : null}
      <form onSubmit={submit} className="flex flex-col gap-4">
        <h2 className="font-semibold">Change password</h2>
        {change.isError ? <Notice tone="error">{describeError(change.error)}</Notice> : null}
        <TextField label="Current password" type="password" autoComplete="current-password" required
          value={form.current} onChange={(e) => setForm({ ...form, current: e.target.value })} />
        <TextField label="New password" type="password" autoComplete="new-password" required minLength={12}
          value={form.next} onChange={(e) => setForm({ ...form, next: e.target.value })} hint="At least 12 characters." />
        <TextField label="New password again" type="password" autoComplete="new-password" required
          value={form.again} onChange={(e) => setForm({ ...form, again: e.target.value })}
          error={mismatch ? "The two passwords differ." : undefined} />
        <p className="text-sm text-ink-soft">Your other browsers are logged out when the password changes.</p>
        <div><Button type="submit" variant="primary" busy={change.isPending}>Change password</Button></div>
      </form>
    </div>
  );
}
