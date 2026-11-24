import { Link, useParams } from "react-router";
import { hasRole, useMe } from "@/features/auth";
import { describeError } from "@/shared/api/errors";
import { EmptyState, Notice } from "@/shared/ui";
import { useSecurity } from "../hooks/useSecurity";
import { TargetCard } from "../components/TargetCard";
import { FindingsTable } from "../components/FindingsTable";

/**
 * The site's security scans by the tenant's scanner: through Equalizer and,
 * for a before and after, the original site. Each finding says what the
 * published map does with it.
 */
export function SecurityTab() {
  const { tenant = "", app = "" } = useParams();
  const me = useMe().data;
  const sec = useSecurity(app);
  const isAdmin = hasRole(me, tenant, "admin");
  if (sec.isError) return <Notice tone="error">{describeError(sec.error)}</Notice>;
  if (!sec.data) return null;
  const { configured, targets } = sec.data;
  const scanned = targets.filter((t) => t.findings.length > 0 || t.scan?.status === "completed");
  return (
    <div className="flex max-w-5xl flex-col gap-6">
      {!configured ? (
        <EmptyState title="No scanner connected">
          {isAdmin ? <>Connect your CVE Radar account in <Link className="text-signal hover:underline" to={`/t/${tenant}/settings`}>Settings → Security scanner</Link>, then scan the site from here.</>
            : "An administrator connects the scanner in Settings; past scans stay readable here."}
        </EmptyState>
      ) : null}
      <div className="grid gap-4 md:grid-cols-2">
        {targets.map((t) => <TargetCard key={t.target} app={app} t={t} canScan={isAdmin} configured={configured} />)}
      </div>
      {scanned.map((t) => (
        <section key={t.target} className="flex flex-col gap-2">
          <h2 className="text-base font-semibold">
            {t.target === "protected" ? "Found through Equalizer" : "Found on the original site"} ({t.findings.length})
          </h2>
          {t.findings.length ? <FindingsTable findings={t.findings} target={t.target} caption={`Findings of ${t.domain ?? t.target}`} />
            : <p className="text-sm text-ink-soft">Nothing open: the last scan found nothing.</p>}
        </section>
      ))}
    </div>
  );
}
