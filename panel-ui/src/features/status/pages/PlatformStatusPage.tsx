import { describeError } from "@/shared/api/errors";
import { Notice, PageHeader, formatDate } from "@/shared/ui";
import { usePlatformStatus } from "../hooks/useStatus";
import { State } from "../components/State";

/** The platform's own services; no tenant data. */
export function PlatformStatusPage() {
  const st = usePlatformStatus();
  return (
    <div className="flex flex-col gap-5">
      <PageHeader title="Platform status"
        subtitle={st.data ? `Checked ${formatDate(st.data.checked_at)}. Refreshed every 30 seconds.` : "Whether the platform's services answer."} />
      {st.isError ? <Notice tone="error">{describeError(st.error)}</Notice> : null}
      <dl className="grid max-w-3xl grid-cols-[10rem_minmax(0,1fr)] gap-x-4 border-t border-rule-soft text-sm">
        {st.data?.services.map((s) => (
          <div key={s.name} className="contents">
            <dt className="border-b border-rule-soft py-3 font-semibold">{s.name}</dt>
            <dd className="flex flex-wrap gap-x-3 border-b border-rule-soft py-3"><State state={s.state} />
              {s.detail ? <span className="text-ink-soft">{s.detail}</span> : null}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
