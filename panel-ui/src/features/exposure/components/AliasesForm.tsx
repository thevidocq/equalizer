import { useState, type FormEvent } from "react";
import { Button, Notice, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useExposureActions } from "../hooks/useExposure";

type Props = { app: string; domain: string; current: string[] };

/** Other domains of the site, redirected to its public domain. */
export function AliasesForm({ app, domain, current }: Props) {
  const { setAliases } = useExposureActions(app);
  const toast = useToast();
  const [text, setText] = useState(current.join("\n"));
  const bare = domain.startsWith("www.") ? domain.slice(4) : `www.${domain}`;

  function submit(e: FormEvent) {
    e.preventDefault();
    const list = text.split(/[\s,;]+/).map((d) => d.trim()).filter(Boolean);
    setAliases.mutate(list, { onSuccess: () => toast("Other domains saved: the gateways apply them within a minute") });
  }

  return (
    <form onSubmit={submit} className="flex max-w-xl flex-col gap-3">
      <div>
        <h2 className="text-base font-semibold">Other domains</h2>
        <p className="mt-1 text-sm text-ink-soft">
          Names of the same site, e.g. <span className="font-mono">{bare}</span> next to <span className="font-mono">{domain}</span>.
          Point them to Equalizer as well: they get a certificate and send visitors to {domain} (permanent redirect), so the
          old server is never reached through them.
        </p>
      </div>
      {setAliases.isError ? <Notice tone="error">{describeError(setAliases.error)}</Notice> : null}
      <label className="flex flex-col gap-1.5 text-sm font-medium">
        Other domains, one per line
        <textarea
          rows={3} value={text} onChange={(e) => setText(e.target.value)} placeholder={bare}
          className="max-w-md rounded-[var(--radius-control)] border border-rule bg-surface px-3 py-2 font-mono text-[13px] font-normal"
        />
        <span className="font-normal text-ink-soft">Up to 10. They apply to the published map at once, without a new version.</span>
      </label>
      <div><Button type="submit" busy={setAliases.isPending}>Save domains</Button></div>
    </form>
  );
}
