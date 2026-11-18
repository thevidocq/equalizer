import { Button, Notice } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useAskReview, useReview, type MapReview } from "../hooks/useReview";

const sev = { high: "border-state-rejected text-state-rejected", medium: "border-state-review text-state-review", low: "border-rule text-ink-soft" } as const;
const change = { added: "+", removed: "−", changed: "~" } as const;

/** The approver's aid: changes, risks found by fixed rules, the AI's summary. It never approves. */
export function ReviewPanel({ app, version, canAsk }: { app: string; version: number; canAsk: boolean }) {
  const review = useReview(app, version);
  const ask = useAskReview(app, version);
  if (review.isError) return <Notice tone="error">{describeError(review.error)}</Notice>;
  const r = review.data;
  if (!r) return null;
  return (
    <section aria-labelledby="review-title" className="flex flex-col gap-4 border border-rule bg-surface p-5">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 id="review-title" className="text-base font-semibold">Review aid</h3>
        <span className="text-sm text-ink-soft">{r.base_version ? `Compared with published version ${r.base_version}` : "No published version yet: everything is new"}</span>
      </div>
      <AI r={r} canAsk={canAsk} onAsk={() => ask.mutate()} busy={ask.isPending} />
      {ask.isError ? <Notice tone="error">{describeError(ask.error)}</Notice> : null}
      <div className="flex flex-col gap-2">
        <h4 className="text-sm font-semibold">Risks ({r.findings.length})</h4>
        {r.findings.length === 0 ? <p className="text-sm text-ink-soft">No rule found a risk in this change.</p> : null}
        <ul className="flex flex-col gap-2">
          {r.findings.map((f, i) => (
            <li key={i} className={`border-l-4 pl-3 text-sm ${sev[f.severity]}`}>
              <span className="font-mono text-[12.5px]">{f.entry}</span>
              <span className="block text-ink">{f.message}</span>
            </li>
          ))}
        </ul>
      </div>
      <details>
        <summary className="cursor-pointer text-sm font-semibold">Changes ({r.changes.length})</summary>
        <ul className="mt-2 max-h-64 overflow-auto font-mono text-[12.5px] leading-relaxed">
          {r.changes.map((c, i) => <li key={i}>{change[c.change]} {c.section}.{c.entry}{c.detail ? `  ${c.detail}` : ""}</li>)}
        </ul>
      </details>
      <p className="text-xs text-ink-soft">The rules and the AI inform; approving stays your decision.</p>
    </section>
  );
}

function AI({ r, canAsk, onAsk, busy }: { r: MapReview; canAsk: boolean; onAsk: () => void; busy: boolean }) {
  const ai = r.ai;
  if (ai.status === "off") return <p className="text-sm text-ink-soft">No AI model configured: only the fixed rules below.</p>;
  if (ai.status === "queued" || ai.status === "running") return <p className="text-sm text-ink-soft">The AI is reading the change…</p>;
  return (
    <div className="flex flex-col gap-2">
      {ai.status === "failed" ? <Notice tone="error">The AI review failed: {ai.error}</Notice> : null}
      {ai.summary ? <p className="max-w-prose text-sm">{ai.summary}</p> : null}
      {ai.questions?.length ? (
        <>
          <h4 className="text-sm font-semibold">Before approving</h4>
          <ul className="list-disc pl-5 text-sm">{ai.questions.map((q, i) => <li key={i}>{q}</li>)}</ul>
        </>
      ) : null}
      {canAsk ? <div><Button onClick={onAsk} busy={busy}>Ask the AI again</Button></div> : null}
    </div>
  );
}
