import type { Schemas } from "@/shared/api/client";

type P = Schemas["VerificationProgress"];

const STEP: Record<P["step"], string> = {
  pages: "comparing pages with and without Equalizer",
  attacks: "trying attacks on the gateway",
  files: "checking the files pages load",
  correcting: "checking its corrected version",
};

/** Where a running verification is, in words: "comparing pages… 120 of 349". */
export function progressText(p: P | null | undefined): string {
  if (!p) return "";
  const of = p.total ? `: ${p.step === "attacks" ? "entry" : "page"} ${Math.min(p.done + 1, p.total)} of ${p.total}` : "";
  return `${STEP[p.step] ?? p.step}${of}${p.account ? ` (as ${p.account})` : ""}`;
}
