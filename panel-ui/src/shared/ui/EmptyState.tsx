import type { ReactNode } from "react";

/** An empty list is an invitation to act: say what goes here and how. */
export function EmptyState({ title, children, action }: { title: string; children?: ReactNode; action?: ReactNode }) {
  return (
    <div className="rounded-[var(--radius-panel)] border border-dashed border-rule px-6 py-10">
      <p className="font-semibold">{title}</p>
      {children ? <div className="mt-1 max-w-prose text-sm text-ink-soft">{children}</div> : null}
      {action ? <div className="mt-4">{action}</div> : null}
    </div>
  );
}
