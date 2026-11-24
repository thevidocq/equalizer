type Seen = { name: string; version?: string | null };
export type Component = { name: string; version: string | null; seen: "pages" | "code" | "pages and code" };

/**
 * One list of the site's components: those the pages show and those the
 * code declares, each once, with where it was seen; the code's version wins
 * (the pages often hide it).
 */
export function mergeComponents(pages: Seen[] = [], code: Seen[] = []): Component[] {
  const out = new Map<string, Component>();
  for (const c of pages) out.set(c.name, { name: c.name, version: c.version ?? null, seen: "pages" });
  for (const c of code) {
    const had = out.get(c.name);
    out.set(c.name, { name: c.name, version: c.version ?? had?.version ?? null, seen: had ? "pages and code" : "code" });
  }
  return [...out.values()].sort((a, b) => a.name.localeCompare(b.name));
}
