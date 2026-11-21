/**
 * One way to write dates and times in the whole panel:
 * "21 Nov 2026, 02:33:07 UTC+1", in the time zone the person chose in their
 * account (UTC until they choose one). The alert emails write them the same
 * way.
 */
let zone = "UTC";

/** Sets the time zone every date is written in (from the logged-in person). */
export function setTimeZone(tz: string | null | undefined) {
  zone = tz && isTimeZone(tz) ? tz : "UTC";
}

export function timeZone(): string {
  return zone;
}

export function isTimeZone(tz: string): boolean {
  try {
    new Intl.DateTimeFormat("en-GB", { timeZone: tz });
    return true;
  } catch {
    return false;
  }
}

/** "UTC", "UTC+1", "UTC+5:30" for a moment in a zone. */
export function zoneLabel(d: Date, tz = zone): string {
  const name = new Intl.DateTimeFormat("en-GB", { timeZone: tz, timeZoneName: "shortOffset" })
    .formatToParts(d).find((p) => p.type === "timeZoneName")?.value ?? "GMT";
  return name.replace(/^GMT/, "UTC");
}

function parts(d: Date, opts: Intl.DateTimeFormatOptions, tz = zone): Record<string, string> {
  const out: Record<string, string> = {};
  for (const p of new Intl.DateTimeFormat("en-GB", { timeZone: tz, hourCycle: "h23", ...opts }).formatToParts(d)) out[p.type] = p.value;
  return out;
}

/** Midnight of a calendar day (YYYY-MM-DD) in the person's time zone. */
export function startOfDay(date: string, tz = zone): Date {
  const guess = new Date(`${date}T00:00:00Z`);
  const p = parts(guess, { year: "numeric", month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" }, tz);
  const seen = Date.UTC(Number(p.year), Number(p.month) - 1, Number(p.day), Number(p.hour), Number(p.minute));
  return new Date(guess.getTime() - (seen - guess.getTime()));
}

/** Today as YYYY-MM-DD in the person's time zone. */
export function today(tz = zone): string {
  const p = parts(new Date(), { year: "numeric", month: "2-digit", day: "2-digit" }, tz);
  return `${p.year}-${p.month}-${p.day}`;
}

/** A date and time for people: 21 Nov 2026, 02:33:07 UTC+1. */
export function formatDate(iso?: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const p = parts(d, { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit", second: "2-digit" });
  return `${p.day} ${p.month} ${p.year}, ${p.hour}:${p.minute}:${p.second} ${zoneLabel(d)}`;
}

/** A short label for chart axes: 02:00, or 21 Nov for whole days. */
export function formatTick(d: Date, days: boolean): string {
  const p = parts(d, days ? { day: "numeric", month: "short" } : { hour: "2-digit", minute: "2-digit" });
  return days ? `${p.day} ${p.month}` : `${p.hour}:${p.minute}`;
}

/** A calendar day (YYYY-MM-DD, already a day): Mon 21. */
export function formatDay(date: string): string {
  return new Date(`${date}T00:00:00Z`).toLocaleDateString("en-GB", { weekday: "short", day: "numeric", timeZone: "UTC" });
}
