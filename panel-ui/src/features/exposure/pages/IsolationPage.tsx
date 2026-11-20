import { useParams } from "react-router";
import { Notice } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { canManage, useMe } from "@/features/auth";
import { useExposureActions, useSite } from "../hooks/useExposure";
import { ExposureStatus } from "../components/ExposureStatus";
import { LegacyIpForm } from "../components/LegacyIpForm";
import { IsolationGuide } from "../components/IsolationGuide";
import { AliasesForm } from "../components/AliasesForm";

/** Site → Isolation: is the site still reachable without Equalizer? */
export function IsolationPage() {
  const { tenant = "", app = "" } = useParams();
  const site = useSite(app).data;
  const me = useMe();
  const { check } = useExposureActions(app);
  if (!site) return null;
  const exposure = site.exposure ?? { status: "not_configured" as const };
  const manage = canManage(me.data, tenant);

  return (
    <div className="flex flex-col gap-8">
      <p className="max-w-prose text-ink-soft">
        A protected site must stop answering on its old address, or an attacker who knows it can skip Equalizer. The other sites
        of the same server can stay public.
      </p>
      {check.isError ? <Notice tone="error">{describeError(check.error)}</Notice> : null}
      <ExposureStatus exposure={exposure} onCheck={manage ? () => check.mutate() : undefined} busy={check.isPending} />
      {manage ? <LegacyIpForm key={`ip:${exposure.legacy_ip ?? ""}`} app={app} current={exposure.legacy_ip} /> : null}
      {manage ? <IsolationGuide domain={site.public_domain} /> : null}
      {manage ? <AliasesForm key={`aliases:${(site.aliases ?? []).join(",")}`} app={app} domain={site.public_domain} current={site.aliases ?? []} /> : null}
    </div>
  );
}
