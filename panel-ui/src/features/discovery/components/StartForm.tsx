import { useState, type FormEvent } from "react";
import { Button, Notice, SelectField, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useConnectors } from "@/features/connectors";
import { useStartDiscovery } from "../hooks/useDiscovery";
import { TestAccounts, type TestAccount } from "@/features/maps";

type Props = { tenant: string; app: string; firstTime: boolean; aiState: "on" | "off" | "unknown"; isAdmin: boolean };

/** Starts a discovery: read-only, through the site's connector. */
export function StartForm({ tenant, app, firstTime, aiState, isAdmin }: Props) {
  const start = useStartDiscovery(app);
  const connectors = useConnectors(tenant);
  const toast = useToast();
  const [useAI, setUseAI] = useState(true);
  const [accounts, setAccounts] = useState<TestAccount[]>([]);
  const [reach, setReach] = useState({ connector_id: "", address: "", scheme: "https" as "https" | "http" });
  const [maxPages, setMaxPages] = useState("500");
  const connector = reach.connector_id || connectors.data?.[0]?.id || "";

  function submit(e: FormEvent) {
    e.preventDefault();
    start.mutate({
      use_ai: useAI, max_pages: Number(maxPages) || 500,
      accounts: accounts.filter((a) => a.username.trim() !== ""),
      ...(firstTime ? { ...reach, connector_id: connector } : {}),
    }, { onSuccess: () => { setAccounts([]); toast("Discovery queued"); } });
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4 border border-rule bg-surface p-5">
      <h2 className="text-base font-semibold">Explore the site</h2>
      <p className="max-w-prose text-sm text-ink-soft">
        The agent reads the site through its connector, never from the Internet. It opens no admin area and follows no link that
        changes something. At the end you get a report and, if the map should change, a draft to review.
      </p>
      {start.isError ? <Notice tone="error">{describeError(start.error)}</Notice> : null}
      <label className="flex items-start gap-2 text-sm">
        <input type="checkbox" className="mt-1" checked={useAI} onChange={(e) => setUseAI(e.target.checked)} />
        <span><span className="font-semibold">Use AI</span> <span className="text-ink-soft">to name routes, spot sensitive pages and narrow types.</span></span>
      </label>
      {useAI && aiState === "off" ? (
        <Notice tone="info">No AI model is configured for your organisation: the discovery runs without it.
          {isAdmin ? <> Set one in <a className="font-semibold text-signal hover:underline" href={`/t/${tenant}/settings`}>Settings</a>.</> : " Ask an administrator."}</Notice>
      ) : null}
      {firstTime ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <SelectField label="Connector" value={connector} onChange={(e) => setReach({ ...reach, connector_id: e.target.value })}>
            {connectors.data?.map((c) => <option key={c.id} value={c.id}>{c.id}</option>)}
          </SelectField>
          <TextField label="Address on the server" required placeholder="127.0.0.1:8080" value={reach.address}
            onChange={(e) => setReach({ ...reach, address: e.target.value })} hint="host:port, as in the connector's EQ_ALLOW" />
          <SelectField label="Protocol" value={reach.scheme} onChange={(e) => setReach({ ...reach, scheme: e.target.value as "https" | "http" })}>
            <option value="https">HTTPS</option><option value="http">HTTP</option>
          </SelectField>
        </div>
      ) : null}
      <TestAccounts value={accounts} onChange={setAccounts} />
      <div className="grid gap-4 sm:grid-cols-3">
        <TextField label="Pages at most" type="number" min={10} max={2000} value={maxPages} onChange={(e) => setMaxPages(e.target.value)} />
      </div>
      <div>
        <Button type="submit" variant="primary" busy={start.isPending} disabled={firstTime && !connector}>Start discovery</Button>
      </div>
    </form>
  );
}
