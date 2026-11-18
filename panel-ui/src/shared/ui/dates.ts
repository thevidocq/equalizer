const fmt = new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" });

/** A date for people, in their locale. */
export function formatDate(iso?: string | null): string {
  return iso ? fmt.format(new Date(iso)) : "";
}
