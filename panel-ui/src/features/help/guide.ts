/** A guide parsed from markdown: # title, ## macro topics, ### topics. */
export type Topic = { id: string; title: string; macroId: string; macroTitle: string; body: string };
export type Macro = { id: string; title: string; topics: Topic[] };
export type Guide = { title: string; intro: string; macros: Macro[]; topics: Topic[] };

export function slug(s: string): string {
  return s.toLowerCase().normalize("NFKD").replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
}

export function parseGuide(md: string): Guide {
  const guide: Guide = { title: "", intro: "", macros: [], topics: [] };
  let macro: Macro | null = null;
  let topic: Topic | null = null;
  let fence = false;
  const intro: string[] = [];
  const ids = new Set<string>();
  const add = (macroTitle: string, title: string): Topic => {
    let id = slug(title) || "topic";
    if (ids.has(id)) id = `${slug(macroTitle)}-${id}`;
    ids.add(id);
    const t: Topic = { id, title, macroId: macro!.id, macroTitle, body: "" };
    macro!.topics.push(t);
    guide.topics.push(t);
    return t;
  };
  for (const line of md.split("\n")) {
    if (line.startsWith("```")) fence = !fence;
    const h = fence ? null : /^(#{1,3}) (.+)$/.exec(line);
    if (h && h[1] === "#") {
      guide.title = h[2]!.trim();
    } else if (h && h[1] === "##") {
      macro = { id: slug(h[2]!), title: h[2]!.trim(), topics: [] };
      guide.macros.push(macro);
      topic = null;
    } else if (h && h[1] === "###") {
      topic = add(macro?.title ?? "", h[2]!.trim());
    } else if (macro && !topic && line.trim() !== "") {
      topic = add(macro.title, macro.title); // text before the first topic
      topic.body += line + "\n";
    } else if (topic) {
      topic.body += line + "\n";
    } else if (!macro) {
      intro.push(line);
    }
  }
  for (const t of guide.topics) t.body = t.body.trim();
  guide.intro = intro.join("\n").trim();
  return guide;
}

/** Plain text of a markdown body: what search reads and snippets show. */
export function plain(md: string): string {
  return md
    .replace(/!\[[^\]]*\]\([^)]*\)/g, "")
    .replace(/\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/[`*|#]/g, "")
    .replace(/(^|\s)>\s/g, "$1")
    .replace(/-{3,}/g, "")
    .replace(/\s+/g, " ")
    .trim();
}

export type Hit = { topic: Topic; snippet: string };

/** Topics holding every word of the query, titles first. */
export function search(guide: Guide, query: string): Hit[] {
  const words = query.toLowerCase().split(/\s+/).filter((w) => w.length > 1);
  if (words.length === 0) return [];
  const hits: (Hit & { score: number })[] = [];
  for (const topic of guide.topics) {
    const title = `${topic.macroTitle} ${topic.title}`.toLowerCase();
    const text = plain(topic.body);
    const lower = text.toLowerCase();
    if (!words.every((w) => title.includes(w) || lower.includes(w))) continue;
    const score = words.filter((w) => title.includes(w)).length;
    const at = Math.max(0, ...words.map((w) => lower.indexOf(w)).filter((i) => i >= 0).slice(0, 1));
    const start = Math.max(0, at - 60);
    const snippet = (start > 0 ? "…" : "") + text.slice(start, start + 180) + (start + 180 < text.length ? "…" : "");
    hits.push({ topic, snippet, score });
  }
  return hits.sort((a, b) => b.score - a.score).map(({ topic, snippet }) => ({ topic, snippet }));
}
