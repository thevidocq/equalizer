import type { Schemas } from "@/shared/api/client";

export type HealthState = Schemas["HealthState"];

const look: Record<HealthState, { label: string; color: string }> = {
  ok: { label: "Working", color: "#2b7a4b" },
  degraded: { label: "Degraded", color: "#a86a12" },
  down: { label: "Down", color: "#b42318" },
  unknown: { label: "Unknown", color: "#6b7684" },
  not_used: { label: "Not used", color: "#9aa3ae" },
};

/** A state as a dot and a word: never colour alone. */
export function State({ state, strong }: { state: HealthState; strong?: boolean }) {
  const l = look[state];
  return (
    <span className={`inline-flex items-center gap-1.5 ${strong ? "font-semibold" : ""}`}>
      <span aria-hidden="true" className="size-2 rounded-full" style={{ background: l.color }} />
      {l.label}
    </span>
  );
}
