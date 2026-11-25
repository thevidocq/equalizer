/** Where an entry of the source study comes from: the extension the reader
 * named, or, for reports written before it named one, the plugin or theme
 * read from the file's path. */
export function owner(file: string, named?: string | null): string {
  if (named) return named;
  const m = /^(?:wp-content\/)?(plugins|themes)\/([^/]+)\//.exec(file);
  if (m && m[1] && m[2]) return `${m[1].replace(/s$/, "")} ${m[2]}`;
  return "the site's own code";
}

/** Items grouped by owner, the largest group first. */
export function byOwner<T>(items: T[], file: (i: T) => string, named: (i: T) => string | null | undefined = () => null): [string, T[]][] {
  const groups = new Map<string, T[]>();
  for (const i of items) {
    const k = owner(file(i), named(i));
    groups.set(k, [...(groups.get(k) ?? []), i]);
  }
  return [...groups.entries()].sort((a, b) => b[1].length - a[1].length || a[0].localeCompare(b[0]));
}
