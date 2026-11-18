import { Button, formatDate } from "@/shared/ui";
import type { App } from "@/shared/api/client";

type Exposure = NonNullable<App["exposure"]>;

const headline: Record<Exposure["status"], string> = {
  not_configured: "Not checked: enter the old public IP below",
  pending: "Waiting for the first check",
  closed: "Not reachable directly",
  exposed: "Still reachable directly, bypassing Equalizer",
  error: "The check could not run",
};

const reasons: Record<string, string> = {
  refused: "nothing answers for this site on the old IP",
  timeout: "the old IP does not answer",
  forbidden: "the server refuses visitors for this site",
  default_site: "the server shows another site for this name",
  redirect_only: "the old address only redirects to the protected one",
  answers_https: "the old IP serves the site on HTTPS",
  answers_http: "the old IP serves the site on HTTP",
  probe_failed: "check the IP address",
};

const tone: Record<Exposure["status"], string> = {
  not_configured: "border-rule", pending: "border-rule", error: "border-state-review",
  closed: "border-state-published", exposed: "border-state-rejected",
};

export function ExposureStatus({ exposure, onCheck, busy }: { exposure: Exposure; onCheck?: () => void; busy?: boolean }) {
  const reason = exposure.reason ? reasons[exposure.reason] ?? exposure.reason : null;
  return (
    <section aria-live="polite" className={`flex flex-wrap items-center justify-between gap-4 border-l-4 bg-surface px-5 py-4 ${tone[exposure.status]}`}>
      <div className="flex flex-col gap-1">
        <h2 className={`text-base font-semibold ${exposure.status === "exposed" ? "text-state-rejected" : ""}`}>{headline[exposure.status]}</h2>
        <p className="text-sm text-ink-soft">
          {reason ? `${reason.charAt(0).toUpperCase()}${reason.slice(1)}. ` : ""}
          {exposure.checked_at ? `Last check ${formatDate(exposure.checked_at)}; checked again every hour.` : null}
        </p>
      </div>
      {onCheck && exposure.legacy_ip ? <Button onClick={onCheck} busy={busy}>Check now</Button> : null}
    </section>
  );
}
