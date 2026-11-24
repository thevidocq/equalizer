import { useState, type FormEvent } from "react";
import { Button, Disclosure, Notice, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { ReachFields, emptyReach, useChosenConnector, type Reach } from "@/features/connectors";
import { useStartDiscovery } from "../hooks/useDiscovery";
import { TestAccounts, type TestAccount } from "@/features/maps";
import { useSource } from "@/features/source";

type Props = { tenant: string; app: string; firstTime: boolean; aiState: "on" | "off" | "unknown"; isAdmin: boolean };

/** Starts a discovery: read-only, through the connector or directly, as the site is reached. */
export function StartForm({ tenant, app, firstTime, aiState, isAdmin }: Props) {
  const start = useStartDiscovery(app);
  const toast = useToast();
  const [accounts, setAccounts] = useState<TestAccount[]>([]);
  const [reach, setReach] = useState<Reach>(emptyReach);
  const [maxPages, setMaxPages] = useState("500");
  const connector = useChosenConnector(tenant, reach);
  const filled = accounts.filter((a) => a.username.trim() !== "").length;
  // checked by default once the source reader answers; grey until then
  const ready = useSource(app).data?.status === "ready";
  const [readSource, setReadSource] = useState<boolean | null>(null);
  const withSource = ready && (readSource ?? true);

  function submit(e: FormEvent) {
    e.preventDefault();
    start.mutate({
      max_pages: Number(maxPages) || 500, mode: reach.mode, source: withSource,
      accounts: accounts.filter((a) => a.username.trim() !== ""),
      ...(firstTime ? { ...reach, connector_id: connector } : {}),
    }, { onSuccess: () => { setAccounts([]); toast("Discovery queued"); } });
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4 border border-rule bg-surface p-5">
      <h2 className="text-base font-semibold">Explore the site</h2>
      <p className="max-w-prose text-sm text-ink-soft">
        The agent reads the site the way Equalizer reaches it, through the connector or directly. It opens no admin area and follows no link that
        changes something. At the end you get a report and, if the map should change, a draft to review.
      </p>
      {start.isError ? <Notice tone="error">{describeError(start.error)}</Notice> : null}
      {aiState === "off" ? (
        <Notice tone="warning">Without AI: no model is configured for your organisation, so routes keep plain names and types
          only what the fixed rules see.
          {isAdmin ? <> Set one in <a className="font-semibold underline" href={`/t/${tenant}/settings`}>Settings → AI model</a>.</> : " Ask an administrator."}</Notice>
      ) : null}
      <label className={`flex items-start gap-2 text-sm ${ready ? "" : "opacity-60"}`}>
        <input type="checkbox" className="mt-1" disabled={!ready} checked={withSource} onChange={(e) => setReadSource(e.target.checked)} />
        <span><span className="font-semibold">Read the site's source code</span>{" "}
          <span className="text-ink-soft">{ready ? "first: fields get the code's types and the report lists what links do not show."
            : <>needs the source reader, set up in the <a className="font-semibold text-signal hover:underline" href={`/t/${tenant}/sites/${app}/source`}>Source</a> tab.</>}</span></span>
      </label>
      {firstTime ? (
        <ReachFields tenant={tenant} value={reach} onChange={setReach} />
      ) : null}
      <Disclosure title="Test accounts" note={filled ? `${filled} set` : "optional"}>
        <TestAccounts value={accounts} onChange={setAccounts} bare />
      </Disclosure>
      <Disclosure title="Pages at most" note={maxPages}>
        <div className="grid gap-4 sm:grid-cols-3">
          <TextField label="Pages at most" type="number" min={10} max={2000} value={maxPages} onChange={(e) => setMaxPages(e.target.value)}
            hint="The agent stops after this many pages." />
        </div>
      </Disclosure>
      <div>
        <Button type="submit" variant="primary" busy={start.isPending} disabled={firstTime && reach.mode === "connector" && !connector}>Start discovery</Button>
      </div>
    </form>
  );
}
