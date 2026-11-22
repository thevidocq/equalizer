import { useState, type FormEvent } from "react";
import { Button, Notice, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useSaveAlerts, type AlertInput } from "../hooks/useAlerts";

type Kinds = NonNullable<AlertInput["kinds"]>;
type Current = { enabled: boolean; extra_emails: string[]; kinds?: Kinds; mail_available?: boolean };

const kindText: [keyof Kinds, string][] = [
  ["connector", "Connector offline"],
  ["legacy_site", "Legacy site failing or not answering"],
  ["isolation", "Site reachable again on its old IP, bypassing Equalizer"],
  ["certificate", "Certificate missing or expired"],
  ["certificate_expiry", "Certificate about to end (30, 14, 7 and 1 days before)"],
  ["recovered", "The site works again after a problem"],
  ["draft_ready", "A discovery's draft is checked and ready to publish"],
];

/** Who gets an email when a site stops working, when it works again and when a draft is ready. */
export function AlertsForm({ tenant, current }: { tenant: string; current: Current }) {
  const save = useSaveAlerts(tenant);
  const toast = useToast();
  const [enabled, setEnabled] = useState(current.enabled);
  const [extra, setExtra] = useState(current.extra_emails.join("\n"));
  const [kinds, setKinds] = useState<Kinds>(current.kinds ?? {});

  function submit(e: FormEvent) {
    e.preventDefault();
    const extra_emails = extra.split(/[\s,;]+/).map((x) => x.trim()).filter(Boolean);
    save.mutate({ enabled, extra_emails, kinds }, { onSuccess: () => toast("Alerts saved") });
  }

  return (
    <form onSubmit={submit} className="flex max-w-xl flex-col gap-4 border-b border-rule-soft pb-6">
      <div>
        <h2 className="text-base font-semibold">Alerts</h2>
        <p className="mt-1 text-sm text-ink-soft">
          An email when something stays wrong for more than a few minutes, or when a draft is ready to
          publish. Choose which.
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
      <fieldset className="flex flex-col gap-2 disabled:opacity-60" disabled={!enabled}>
        <legend className="mb-1 text-sm font-medium">Send an email when</legend>
        {kindText.map(([k, text]) => (
          <label key={k} className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={kinds[k] !== false} onChange={(e) => setKinds({ ...kinds, [k]: e.target.checked })} />
            {text}
          </label>
        ))}
      </fieldset>
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
