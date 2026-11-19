import { useState, type FormEvent } from "react";
import { Button, Notice, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useSaveAlerts } from "../hooks/useAlerts";

type Current = { enabled: boolean; extra_emails: string[]; mail_available?: boolean };

/** Who gets an email when a site stops working, and when it works again. */
export function AlertsForm({ tenant, current }: { tenant: string; current: Current }) {
  const save = useSaveAlerts(tenant);
  const toast = useToast();
  const [enabled, setEnabled] = useState(current.enabled);
  const [extra, setExtra] = useState(current.extra_emails.join("\n"));

  function submit(e: FormEvent) {
    e.preventDefault();
    const extra_emails = extra.split(/[\s,;]+/).map((x) => x.trim()).filter(Boolean);
    save.mutate({ enabled, extra_emails }, { onSuccess: () => toast("Alerts saved") });
  }

  return (
    <form onSubmit={submit} className="flex max-w-xl flex-col gap-4 border-b border-rule-soft pb-6">
      <div>
        <h2 className="text-base font-semibold">Alerts</h2>
        <p className="mt-1 text-sm text-ink-soft">
          An email when a site stops working (connector offline, legacy site failing, site reachable again on its old IP)
          for more than a few minutes, and when it works again.
        </p>
      </div>
      {current.mail_available === false ? (
        <Notice tone="info">This installation sends no email yet: alerts start once the platform sets up its mail server.</Notice>
      ) : null}
      {save.isError ? <Notice tone="error">{describeError(save.error)}</Notice> : null}
      <label className="flex items-center gap-2 text-sm font-semibold">
        <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
        Email the administrators of this organisation
      </label>
      <label className="flex flex-col gap-1.5 text-sm font-medium">
        Also send to
        <textarea
          rows={3} value={extra} onChange={(e) => setExtra(e.target.value)} disabled={!enabled}
          placeholder="oncall@example.com"
          className="max-w-md rounded-[var(--radius-control)] border border-rule bg-surface px-3 py-2 font-normal disabled:opacity-60"
        />
        <span className="font-normal text-ink-soft">Up to 5 addresses, one per line: an on-call mailbox, a ticketing address.</span>
      </label>
      <div><Button type="submit" variant="primary" busy={save.isPending}>Save alerts</Button></div>
    </form>
  );
}
