import type { ReactNode } from "react";

/** A section closed by default: its title, with an arrow, opens it downwards. */
export function Disclosure({ title, note, children }: { title: ReactNode; note?: ReactNode; children: ReactNode }) {
  return (
    <details className="group">
      <summary className="flex cursor-pointer list-none items-center gap-2 text-sm font-semibold [&::-webkit-details-marker]:hidden">
        <svg viewBox="0 0 24 24" className="size-4 text-ink-soft transition-transform group-open:rotate-90" fill="none" stroke="currentColor"
          strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M9 6l6 6-6 6" /></svg>
        <span>{title}</span>
        {note ? <span className="font-normal text-ink-soft">{note}</span> : null}
      </summary>
      <div className="mt-3 pl-6">{children}</div>
    </details>
  );
}
