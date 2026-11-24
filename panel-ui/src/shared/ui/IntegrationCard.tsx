import type { ReactNode } from "react";

export type IntegrationState = "connected" | "not_connected" | "error";

const states: Record<IntegrationState, { color: string; text: string }> = {
  connected: { color: "#2b7a4b", text: "text-state-published" },
  not_connected: { color: "#9aa3ae", text: "text-ink-soft" },
  error: { color: "#b42318", text: "text-state-rejected" },
};

type Props = { logo: string; name: string; description: string; state: IntegrationState; label: string; note?: ReactNode };

/** A service the panel works with: who provides it, and whether it answers. Discreet: a dot and a word. */
export function IntegrationCard({ logo, name, description, state, label, note }: Props) {
  const s = states[state];
  return (
    <div className="rounded-[var(--radius-panel)] border border-rule bg-surface px-4 py-3">
      <div className="flex items-center gap-3">
        <img src={logo} alt="" width={28} height={28} className="shrink-0" />
        <div className="min-w-0">
          <div className="text-sm font-semibold">{name}</div>
          <div className="text-[13px] text-ink-soft">{description}</div>
        </div>
        <span className={`ml-auto inline-flex shrink-0 items-center gap-1.5 text-[13px] font-medium ${s.text}`}>
          <span aria-hidden="true" className="size-2 rounded-full" style={{ background: s.color }} />
          {label}
        </span>
      </div>
      {note ? <p className="mt-2 text-[13px] text-ink-soft">{note}</p> : null}
    </div>
  );
}
