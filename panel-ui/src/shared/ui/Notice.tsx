import type { ReactNode } from "react";

type Tone = "error" | "info" | "success" | "warning";

const tones: Record<Tone, string> = {
  error: "border-state-rejected/40 bg-[#fbeeed] text-state-rejected",
  info: "border-signal/30 bg-signal-soft text-ink",
  success: "border-state-published/40 bg-[#e9f4ee] text-state-published",
  // soft amber: something works less well than it could, nothing failed
  warning: "border-state-review/30 bg-[#fdf6ea] text-[#7a4c0c]",
};

/** An inline message. Errors say what happened and what to do. */
export function Notice({ tone = "info", children }: { tone?: Tone; children: ReactNode }) {
  return (
    <div role={tone === "error" ? "alert" : "status"} className={`rounded-[var(--radius-control)] border px-3 py-2 text-sm ${tones[tone]}`}>
      {children}
    </div>
  );
}
