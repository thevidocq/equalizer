import { Button, Notice } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useCertificateActions } from "../hooks/useCertificate";

/** How visitors reach the site: https with its certificate, or http as before. */
export function SchemeForm({ app, scheme }: { app: string; scheme: "https" | "http" }) {
  const { setScheme } = useCertificateActions(app);
  const other = scheme === "https" ? "http" : "https";
  return (
    <section className="flex max-w-xl flex-col gap-3">
      <div>
        <h2 className="text-base font-semibold">Served in {scheme}</h2>
        <p className="mt-1 text-sm text-ink-soft">
          {scheme === "https"
            ? "Visitors reach the site in https with its own certificate; http is redirected to https."
            : "The site stays in http, as it was before Equalizer: logins and pages travel in clear. Sessions still go through Equalizer, without the protections of https, and the login with an identity provider is not available."}
        </p>
      </div>
      {setScheme.isError ? <Notice tone="error">{describeError(setScheme.error)}</Notice> : null}
      <div>
        <Button variant="secondary" busy={setScheme.isPending} onClick={() => setScheme.mutate(other)}>
          Serve it in {other}
        </Button>
      </div>
    </section>
  );
}
