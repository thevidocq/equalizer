import { Link, useParams } from "react-router";
import { Button, Notice, formatDate } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { hasRole, useMe } from "@/features/auth";
import { useMapVersion } from "../hooks/useMaps";
import { LifecycleRail, StatusLabel } from "../components/LifecycleRail";
import { VersionActions } from "../components/VersionActions";
import { toEditorYaml } from "../components/yaml";
import { ReviewPanel } from "../components/ReviewPanel";
import { VerificationPanel } from "../components/VerificationPanel";
import { exportMap } from "../components/mapFile";

export function VersionPage() {
  const { tenant = "", app = "", version = "0" } = useParams();
  const v = useMapVersion(app, Number(version));
  const me = useMe();
  if (v.isError) return <Notice tone="error">{describeError(v.error)}</Notice>;
  if (!v.data || !me.data) return null;
  const canEdit = hasRole(me.data, tenant, "admin", "editor");

  return (
    <div className="grid gap-8 lg:grid-cols-[1fr_18rem]">
      <section className="flex min-w-0 flex-col gap-4" aria-labelledby="doc-title">
        <div className="flex items-baseline justify-between gap-4">
          <h2 id="doc-title" className="text-lg font-semibold">Version {v.data.version}</h2>
          <Link to=".." relative="path" className="text-sm text-signal hover:underline">All versions</Link>
        </div>
        <LifecycleRail v={v.data} />
        {v.data.status !== "retired" ? (
          <VerificationPanel app={app} version={v.data.version} canVerify={hasRole(me.data, tenant, "admin", "editor", "approver")} />
        ) : null}
        {v.data.status !== "retired" ? (
          <ReviewPanel app={app} version={v.data.version} canAsk={hasRole(me.data, tenant, "admin", "editor", "approver")} />
        ) : null}
        <pre
          tabIndex={0} aria-label="Map document"
          className="scroll-box max-h-[34rem] overflow-auto rounded-[var(--radius-panel)] border border-rule bg-surface p-4 font-mono text-[13px] leading-[1.6]"
        >
          {toEditorYaml(v.data.document as Record<string, unknown>)}
        </pre>
      </section>
      <aside className="flex flex-col gap-5 lg:pt-10">
        <StatusLabel status={v.data.status} />
        {v.data.note ? <p className="text-sm">{v.data.note}</p> : null}
        <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm">
          <dt className="text-ink-soft">Author</dt><dd>{v.data.author}</dd>
          <dt className="text-ink-soft">Saved</dt><dd>{formatDate(v.data.created_at)}</dd>
          {v.data.approver ? (<><dt className="text-ink-soft">Decided by</dt><dd>{v.data.approver}</dd></>) : null}
          {v.data.published_at ? (<><dt className="text-ink-soft">Published</dt><dd>{formatDate(v.data.published_at)}</dd></>) : null}
        </dl>
        <VersionActions v={v.data} tenant={tenant} me={me.data} />
        <div>
          <Button onClick={() => exportMap(app, v.data.version, v.data.document as Record<string, unknown>)}>Download as YAML</Button>
        </div>
        {canEdit ? (
          <Link to={`../new?from=${v.data.version}`} relative="path" className="text-sm text-signal hover:underline">
            Start a new version from this one
          </Link>
        ) : null}
      </aside>
    </div>
  );
}
