import { Link, useParams } from "react-router";
import { EmptyState, Notice, formatDate } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { hasRole, useMe } from "@/features/auth";
import { useMapVersions } from "../hooks/useMaps";
import { LifecycleRail, StatusLabel } from "../components/LifecycleRail";

export function MapsPage() {
  const { tenant = "", app = "" } = useParams();
  const versions = useMapVersions(app);
  const me = useMe();
  const canEdit = hasRole(me.data, tenant, "admin");
  const newLink = canEdit ? (
    <Link to="new" className="rounded-[var(--radius-control)] bg-signal px-3.5 py-2 text-sm font-semibold text-white hover:bg-[#183f94]">
      New version
    </Link>
  ) : null;

  if (versions.isError) return <Notice tone="error">{describeError(versions.error)}</Notice>;
  if (!versions.data) return null;
  if (versions.data.length === 0) {
    const emptyAction = canEdit ? (
      <div className="flex flex-wrap items-center gap-3">
        <Link to="../discovery" relative="path"
          className="rounded-[var(--radius-control)] bg-signal px-3.5 py-2 text-sm font-semibold text-white hover:bg-[#183f94]">
          Run discovery
        </Link>
        <Link to="new" className="text-sm font-semibold text-signal hover:underline">or build one by hand</Link>
      </div>
    ) : null;
    return (
      <EmptyState title="No map yet" action={emptyAction}>
        The map lists everything the gateway lets through: pages, forms, assets. Nothing else reaches the legacy server.
        Start with Discovery: the agent explores the site and proposes a draft for you to review.
      </EmptyState>
    );
  }
  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between gap-4">
        <p className="text-ink-soft">Every change is a new version: publish it when it is right, go back to an earlier one if needed.</p>
        {newLink}
      </div>
      <ol className="flex flex-col">
        {versions.data.map((v) => (
          <li key={v.version} className="border-b border-rule-soft">
            <Link to={String(v.version)} className="grid grid-cols-[7rem_1fr_minmax(12rem,22rem)] items-center gap-6 py-4 hover:bg-surface">
              <span className="pl-2 font-semibold">Version {v.version}</span>
              <span className="min-w-0">
                <StatusLabel status={v.status} />
                <span className="block truncate text-sm text-ink-soft">{v.note || `Saved ${formatDate(v.created_at)}`}</span>
              </span>
              <LifecycleRail v={v} compact />
            </Link>
          </li>
        ))}
      </ol>
    </div>
  );
}
