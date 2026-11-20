import { useConnectorSetup } from "@/features/connectors";

type Props = { tenant: string; domain: string; scheme: string };

/**
 * A site reached directly stays on the Internet and accepts only Equalizer's
 * address. Equalizer cannot test that itself (its own requests are the ones
 * accepted): the check is run from another network.
 */
export function DirectIsolation({ tenant, domain, scheme }: Props) {
  const from = useConnectorSetup(tenant).data?.directFrom ?? [];
  const port = scheme === "http" ? 80 : 443;
  const cmd = `curl -m 10 -sv ${scheme}://<site address>:${port}/ -H "Host: ${domain}" -o /dev/null`;
  return (
    <section className="flex flex-col gap-3 border-l-4 border-state-review bg-surface px-5 py-4">
      <h2 className="text-base font-semibold">Reached directly: test it from another network</h2>
      <p className="max-w-prose text-sm text-ink-soft">
        The site must answer only {from.length > 0 ? <span className="font-mono">{from.join(", ")}</span> : "Equalizer's address"} and
        refuse everyone else. Equalizer cannot check this itself, because its own requests are the ones accepted. From a computer on
        another network (a phone hotspot will do), run:
      </p>
      <pre className="whitespace-pre-wrap break-all rounded-[var(--radius-control)] bg-ink p-3 font-mono text-[12.5px] text-paper">{cmd}</pre>
      <p className="max-w-prose text-sm text-ink-soft">
        Isolated: the connection is refused or times out. Not isolated: the site answers, and anyone who knows its address can skip
        Equalizer.
      </p>
    </section>
  );
}
