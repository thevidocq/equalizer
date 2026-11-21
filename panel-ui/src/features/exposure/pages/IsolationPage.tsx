import { useParams } from "react-router";
import { Notice } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { canManage, useMe } from "@/features/auth";
import { useExposureActions, useSite } from "../hooks/useExposure";
import { ExposureStatus } from "../components/ExposureStatus";
import { LegacyIpForm } from "../components/LegacyIpForm";
import { IsolationGuide } from "../components/IsolationGuide";
import { AliasesForm } from "../components/AliasesForm";
import { DirectIsolation } from "../components/DirectIsolation";
import { ReachForm } from "../components/ReachForm";

/** Site → Isolation: is the site still reachable without Equalizer? */
export function IsolationPage() {
  const { tenant = "", app = "" } = useParams();
  const site = useSite(app).data;
  const me = useMe();
  const { check } = useExposureActions(app);
  if (!site) return null;
  const exposure = site.exposure ?? { status: "not_configured" as const };
  const manage = canManage(me.data, tenant);
  const direct = site.reach_mode === "direct";

  return (
    <div className="flex flex-col gap-8">
      <p className="max-w-prose text-ink-soft">
        A protected site must stop answering on its old address, or an attacker who knows it can skip Equalizer. The other sites
        of the same server can stay public.
      </p>
      {check.isError ? <Notice tone="error">{describeError(check.error)}</Notice> : null}
      {manage && site.published_version != null ? <ReachForm key={`reach:${site.reach_mode}:${site.reach_address}`} tenant={tenant} site={site} /> : null}
      {direct ? <DirectIsolation tenant={tenant} domain={site.public_domain} scheme={site.tls?.public_scheme === "http" ? "http" : "https"} /> : (
        <ExposureStatus exposure={exposure} onCheck={manage ? () => check.mutate() : undefined} busy={check.isPending} />
      )}
      {manage && !direct ? <LegacyIpForm key={`ip:${exposure.legacy_ip ?? ""}:${exposure.status}`} app={app} current={exposure.legacy_ip}
        neverPublic={exposure.status === "never_public"} /> : null}
      {manage && !direct && exposure.status !== "never_public" ? <IsolationGuide domain={site.public_domain} /> : null}
      {manage ? <AliasesForm key={`aliases:${(site.aliases ?? []).join(",")}`} app={app} domain={site.public_domain} current={site.aliases ?? []} /> : null}
    </div>
  );
}
