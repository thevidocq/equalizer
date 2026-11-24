import type { SecurityFinding } from "../hooks/useSecurity";

type Verdict = SecurityFinding["map"]["verdict"];
type Severity = SecurityFinding["severity"];

/** What the map does with a finding's address, in a few words, and why. */
export const verdictText: Record<Verdict, { label: string; tone: "good" | "warn" | "plain"; why: string }> = {
  refused: { label: "Closed", tone: "good", why: "Equalizer refuses this request: nothing in the map serves it, or the entry does not take this input or value." },
  precise: { label: "Checked", tone: "good", why: "A precise entry serves it and checks this input by its type." },
  free_text: { label: "Free text", tone: "warn", why: "A precise entry serves it, but this input is free text: length, markup and rules are checked, not its shape. Give it a pattern or a type if you can." },
  area: { label: "Area only", tone: "warn", why: "Only the generic area serves this request (no precise entry, or one that does not take this value or query): it is rebuilt but not checked precisely. Add or widen a precise entry." },
  asset: { label: "File", tone: "plain", why: "A static file the map serves as it is." },
  server: { label: "Server", tone: "plain", why: "About the server (ports, TLS, headers), not an address: the map has no say." },
};

/**
 * The verdict of a finding of one target. Found through Equalizer, an
 * address the current map refuses was not refused when it was scanned: the
 * map changed since, or the scanner reports it in another form.
 */
export function verdictOf(f: SecurityFinding, target: "protected" | "original") {
  if (target === "protected" && f.map.verdict === "refused") {
    return { label: "Closed now", tone: "plain" as const,
      why: "The scanner reached it through Equalizer, but the current map refuses it: the map changed since the scan, or the scanner reports the address in another form. Scan again to confirm." };
  }
  return verdictText[f.map.verdict];
}

export const severityOrder: Severity[] = ["critical", "high", "medium", "low", "info"];

export const severityStyle: Record<Severity, string> = {
  critical: "bg-[#7a1f1f] text-white",
  high: "bg-state-rejected text-white",
  medium: "bg-state-review text-white",
  low: "bg-rule-soft text-ink",
  info: "border border-rule-soft text-ink-soft",
};

/** How many findings of each severity, the most severe first; none left out. */
export function bySeverity(findings: SecurityFinding[]): { severity: Severity; count: number }[] {
  return severityOrder
    .map((severity) => ({ severity, count: findings.filter((f) => f.severity === severity).length }))
    .filter((c) => c.count > 0);
}

/**
 * The findings Equalizer leaves open: those the map serves without a precise
 * check of the input (free text, area). Closed and checked ones are not.
 */
export function stillOpen(findings: SecurityFinding[]): number {
  return findings.filter((f) => f.map.verdict === "free_text" || f.map.verdict === "area").length;
}

/** The findings the map closes: their address is refused, or its input checked by type. */
export function closedByMap(findings: SecurityFinding[]): number {
  return findings.filter((f) => f.map.verdict === "refused" || f.map.verdict === "precise").length;
}

/** The path and query of a finding's address, without the scheme and host. */
export function shortURL(url: string | null | undefined): string {
  if (!url) return "";
  try {
    const u = new URL(url);
    return u.pathname + u.search;
  } catch {
    return url;
  }
}
