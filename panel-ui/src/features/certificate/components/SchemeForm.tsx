import { useState } from "react";
import { Button, Notice } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useCertificateActions, type Scheme } from "../hooks/useCertificate";

const choices: { value: Scheme; title: string; text: string }[] = [
  {
    value: "https", title: "HTTPS with the site's certificate",
    text: "Visitors reach the site in https with its own certificate, uploaded above; http is redirected to https.",
  },
  {
    value: "https_proxy", title: "HTTPS handled by a proxy in front",
    text: "A CDN, load balancer or reverse proxy in front of Equalizer gives visitors https and reaches Equalizer in http (Cloudflare \"Flexible\", for example). Equalizer writes every address in https and needs no certificate. Between the proxy and Equalizer the traffic is in clear.",
  },
  {
    value: "http", title: "HTTP, as before",
    text: "The site stays in http, as it was before Equalizer: logins and pages travel in clear, and the login with an identity provider is not available.",
  },
];

/** How visitors reach the site. */
export function SchemeForm({ app, scheme }: { app: string; scheme: Scheme }) {
  const { setScheme } = useCertificateActions(app);
  const [chosen, setChosen] = useState<Scheme>(scheme);
  return (
    <section className="flex max-w-xl flex-col gap-3">
      <h2 className="text-base font-semibold">How visitors reach the site</h2>
      {setScheme.isError ? <Notice tone="error">{describeError(setScheme.error)}</Notice> : null}
      <fieldset className="flex flex-col gap-3">
        <legend className="sr-only">How visitors reach the site</legend>
        {choices.map((c) => (
          <label key={c.value} className="flex gap-3 text-sm">
            <input type="radio" name="scheme" className="mt-1" checked={chosen === c.value} onChange={() => setChosen(c.value)} />
            <span>
              <span className="font-medium">{c.title}</span>{c.value === scheme ? <span className="text-ink-soft"> (now)</span> : null}
              <span className="block text-ink-soft">{c.text}</span>
            </span>
          </label>
        ))}
      </fieldset>
      <div>
        <Button variant="secondary" busy={setScheme.isPending} disabled={chosen === scheme} onClick={() => setScheme.mutate(chosen)}>
          Apply
        </Button>
      </div>
    </section>
  );
}
