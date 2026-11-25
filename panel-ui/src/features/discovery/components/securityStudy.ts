import type { Schemas } from "@/shared/api/client";

export type Study = Schemas["SecurityStudy"];
export type Finding = NonNullable<Study["findings"]>[number];

/** How serious, discreetly: a dot's colour and a word. */
export const SEVERITY: Record<Finding["severity"], { color: string; word: string }> = {
  critical: { color: "#b42318", word: "Critical" },
  high: { color: "#d4622a", word: "High" },
  medium: { color: "#c9a227", word: "Medium" },
  low: { color: "#9aa3ae", word: "Low" },
};

export const COVERAGE: Record<Finding["coverage"], string> = {
  open: "open once published",
  partial: "partly stopped by the map",
  blocked: "stopped by the map",
};

export const ACTION: Record<Finding["action"], string> = {
  narrow_field: "Narrow a field",
  exclude_entry: "Take the entry out of the map",
  add_role: "Restrict to a role",
  owner_check: "Check the owner (IDOR)",
  update_component: "Update the component",
  review_code: "Read the code",
  close_area: "Close the area",
  none: "Nothing to do in the map",
};

/** The attack surface in one line. */
export function surfaceLine(s: Study["surface"]): string {
  if (!s) return "";
  const parts = [
    `${s.pages ?? 0} pages`, `${s.forms ?? 0} forms`, s.api ? `${s.api} API entries` : "",
    s.uploads ? `${s.uploads} file upload${s.uploads === 1 ? "" : "s"}` : "", `${s.free_text_fields ?? 0} free-text fields`,
    s.code_entries ? `${s.code_entries} entry points only in the code (${s.code_entries_public ?? 0} without login)` : "",
    s.risky_calls ? `${s.risky_calls} risky calls in the code` : "",
  ];
  return parts.filter(Boolean).join(" · ");
}
