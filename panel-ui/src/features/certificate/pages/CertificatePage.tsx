import { useParams } from "react-router";
import { Button, Notice } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { canManage, useMe } from "@/features/auth";
import { useSite } from "@/features/exposure";
import { useCertificateActions } from "../hooks/useCertificate";
import { CertificateStatus } from "../components/CertificateStatus";
import { CertificateUpload } from "../components/CertificateUpload";
import { SchemeForm } from "../components/SchemeForm";

/** Site → Certificate: the legacy site's own certificate, uploaded here. */
export function CertificatePage() {
  const { tenant = "", app = "" } = useParams();
  const site = useSite(app).data;
  const me = useMe();
  const { remove } = useCertificateActions(app);
  if (!site?.tls) return null;
  const manage = canManage(me.data, tenant);
  const https = site.tls.public_scheme === "https";

  return (
    <div className="flex flex-col gap-8">
      <p className="max-w-prose text-ink-soft">
        Equalizer never makes a certificate for a protected site: visitors see the site&apos;s own, the one its server uses today.
      </p>
      <CertificateStatus tls={site.tls} />
      {manage && https ? <CertificateUpload app={app} domain={site.public_domain} replace={!!site.tls.certificate} /> : null}
      {manage && https && site.tls.certificate ? (
        <div>
          {remove.isError ? <Notice tone="error">{describeError(remove.error)}</Notice> : null}
          <Button variant="secondary" busy={remove.isPending} onClick={() => remove.mutate()}>Remove the certificate</Button>
        </div>
      ) : null}
      {manage ? <SchemeForm app={app} scheme={site.tls.public_scheme} /> : null}
    </div>
  );
}
