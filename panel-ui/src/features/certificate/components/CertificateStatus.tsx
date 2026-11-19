import { formatDate } from "@/shared/ui";
import type { App } from "@/shared/api/client";

type TLS = NonNullable<App["tls"]>;

const headline: Record<TLS["status"], string> = {
  ok: "Certificate in place",
  missing: "No certificate: visitors get no HTTPS",
  incomplete: "The certificate does not cover every domain of the site",
  expiring: "The certificate ends soon: upload the renewed one",
  expired: "The certificate has expired: browsers refuse the site",
  not_needed: "Served in http, as before Equalizer: no certificate",
};

const tone: Record<TLS["status"], string> = {
  ok: "border-state-published", not_needed: "border-rule", incomplete: "border-state-review",
  expiring: "border-state-review", missing: "border-state-rejected", expired: "border-state-rejected",
};

/** What visitors get: the site's certificate, its domains and its end. */
export function CertificateStatus({ tls }: { tls: TLS }) {
  const c = tls.certificate;
  const bad = tls.status === "missing" || tls.status === "expired";
  return (
    <section aria-live="polite" className={`flex flex-col gap-2 border-l-4 bg-surface px-5 py-4 ${tone[tls.status]}`}>
      <h2 className={`text-base font-semibold ${bad ? "text-state-rejected" : ""}`}>{headline[tls.status]}</h2>
      {c && tls.public_scheme === "https" ? (
        <dl className="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-sm">
          <dt className="text-ink-soft">Domains</dt>
          <dd className="font-mono text-[13px]">{c.domains.join(", ")}</dd>
          {c.missing?.length ? (<>
            <dt className="text-ink-soft">Not covered</dt>
            <dd className="font-mono text-[13px] text-state-rejected">{c.missing.join(", ")}: no HTTPS on these</dd>
          </>) : null}
          <dt className="text-ink-soft">Issued by</dt>
          <dd>{c.issuer}</dd>
          <dt className="text-ink-soft">Valid until</dt>
          <dd>{formatDate(c.not_after)}</dd>
          <dt className="text-ink-soft">Uploaded</dt>
          <dd>{formatDate(c.uploaded_at)}{c.uploaded_by ? ` by ${c.uploaded_by}` : ""}</dd>
          <dt className="text-ink-soft">SHA-256</dt>
          <dd className="break-all font-mono text-[12px] text-ink-soft">{c.fingerprint_sha256}</dd>
        </dl>
      ) : null}
    </section>
  );
}
