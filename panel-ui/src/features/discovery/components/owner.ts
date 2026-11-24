/** The plugin, theme or part of the site a file belongs to, from its path. */
export function owner(file: string): string {
  const m = /^(?:wp-content\/)?(plugins|themes|modules|components|extensions)\/([^/]+)\//.exec(file);
  if (m && m[1] && m[2]) return `${m[1].replace(/s$/, "")} ${m[2]}`;
  return "the site's own code";
}

/** Items grouped by owner, the largest group first. */
export function byOwner<T>(items: T[], file: (i: T) => string): [string, T[]][] {
  const groups = new Map<string, T[]>();
  for (const i of items) {
    const k = owner(file(i));
    groups.set(k, [...(groups.get(k) ?? []), i]);
  }
  return [...groups.entries()].sort((a, b) => b[1].length - a[1].length || a[0].localeCompare(b[0]));
}
