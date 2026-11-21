import type { MapVersion } from "@/shared/api/client";

type Step = { label: string; who?: string | null };
type State = "done" | "current" | "todo" | "failed";

const reached: Record<MapVersion["status"], number> = {
  draft: 0, in_review: 0, approved: 0, rejected: 1, published: 1, retired: 1,
};

const stateColor: Record<MapVersion["status"], string> = {
  draft: "bg-state-draft", in_review: "bg-state-review", approved: "bg-state-approved",
  rejected: "bg-state-rejected", published: "bg-state-published", retired: "bg-state-retired",
};

/**
 * The path of a map version: written, then published (or discarded). Every
 * step reached is filled in the colour of the version's state.
 */
export function LifecycleRail({ v, compact = false }: { v: MapVersion; compact?: boolean }) {
  const steps: Step[] = [
    { label: "Draft", who: v.author },
    { label: v.status === "rejected" ? "Discarded" : v.status === "retired" ? "Retired" : "Published", who: v.approver },
  ];
  const last = reached[v.status];
  const state = (i: number): State => {
    if (v.status === "rejected" && i === 1) return "failed";
    if (i < last || (i === last && (v.status === "published" || v.status === "retired"))) return "done";
    return i === last ? "current" : "todo";
  };
  const fill = stateColor[v.status];

  return (
    <ol className="grid grid-cols-2 gap-1" aria-label={`Version ${v.version}: ${v.status.replace("_", " ")}`}>
      {steps.map((s, i) => {
        const st = state(i);
        return (
          <li key={s.label} className="flex min-w-0 flex-col gap-1.5" aria-current={st === "current" ? "step" : undefined}>
            <span className={`h-1.5 rounded-full ${st === "todo" ? "bg-rule-soft" : st === "failed" ? "bg-state-rejected" : fill}
              ${st === "current" ? "opacity-60" : ""}`} />
            {compact ? null : (
              <span className={`truncate text-xs ${st === "todo" ? "text-state-retired" : "text-ink"}`}>
                {s.label}
                {s.who && st !== "todo" ? <span className="text-ink-soft"> by {s.who}</span> : null}
              </span>
            )}
          </li>
        );
      })}
    </ol>
  );
}

export function StatusLabel({ status }: { status: MapVersion["status"] }) {
  const text = { draft: "Draft", in_review: "In review", approved: "Approved", rejected: "Discarded", published: "Published", retired: "Retired" }[status];
  return (
    <span className="inline-flex items-center gap-1.5 text-sm font-semibold">
      <span aria-hidden="true" className={`h-2 w-2 rounded-full ${stateColor[status]}`} />
      {text}
    </span>
  );
}
