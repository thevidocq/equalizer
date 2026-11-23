// A small Markdown reader for the assistant's answers. It returns data, not
// HTML: the answer can quote a path an attacker chose, and React renders
// every piece as text. Links survive only toward the panel itself.

export type Inline =
  | { t: "text"; v: string }
  | { t: "code"; v: string }
  | { t: "bold"; c: Inline[] }
  | { t: "em"; c: Inline[] }
  | { t: "link"; v: string; to: string };

export type Block =
  | { t: "p"; c: Inline[] }
  | { t: "h"; c: Inline[] }
  | { t: "ul" | "ol"; items: Inline[][] }
  | { t: "code"; v: string }
  | { t: "table"; head: Inline[][]; rows: Inline[][][] };

const token = /(`[^`\n]+`)|(\*\*[^*\n]+\*\*|__[^_\n]+__)|(\*[^*\n]+\*|\b_[^_\n]+_\b)|(\[[^\]\n]+\]\([^)\s]+\))/g;

/** Only panel pages: /t/<tenant>/… */
export const internal = (url: string) => /^\/t\/[a-z0-9-]+\/[A-Za-z0-9/_.?=&%-]*$/.test(url) && !url.includes("..");

export function inline(s: string): Inline[] {
  const out: Inline[] = [];
  let last = 0;
  for (const m of s.matchAll(token)) {
    const at = m.index ?? 0;
    if (at > last) out.push({ t: "text", v: s.slice(last, at) });
    const w = m[0];
    if (m[1]) out.push({ t: "code", v: w.slice(1, -1) });
    else if (m[2]) out.push({ t: "bold", c: inline(w.slice(2, -2)) });
    else if (m[3]) out.push({ t: "em", c: inline(w.slice(1, -1)) });
    else {
      const [, text = "", url = ""] = /^\[([^\]]+)\]\(([^)]+)\)$/.exec(w) ?? [];
      out.push(internal(url) ? { t: "link", v: text, to: url } : { t: "text", v: text });
    }
    last = at + w.length;
  }
  if (last < s.length) out.push({ t: "text", v: s.slice(last) });
  return out;
}

const cells = (line: string) => line.trim().replace(/^\|/, "").replace(/\|$/, "").split("|").map((c) => inline(c.trim()));
const bullet = /^\s*[-*•]\s+/;
const numbered = /^\s*\d+[.)]\s+/;
const heading = /^#{1,6}\s+([^\n]*)$/;
const separator = /^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$/;

export function parse(text: string): Block[] {
  const lines = text.replace(/\r\n|[\r\u2028\u2029]/g, "\n").split("\n");
  const blocks: Block[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i] ?? "";
    if (!line.trim()) { i++; continue; }
    if (line.trim().startsWith("```")) {
      const body: string[] = [];
      for (i++; i < lines.length && !(lines[i] ?? "").trim().startsWith("```"); i++) body.push(lines[i] ?? "");
      blocks.push({ t: "code", v: body.join("\n") });
      i++;
      continue;
    }
    const h = heading.exec(line);
    if (h) { blocks.push({ t: "h", c: inline(h[1] ?? "") }); i++; continue; }
    if (line.trim().startsWith("|") && separator.test(lines[i + 1] ?? "")) {
      const head = cells(line);
      const rows: Inline[][][] = [];
      for (i += 2; i < lines.length && (lines[i] ?? "").trim().startsWith("|"); i++) rows.push(cells(lines[i] ?? ""));
      blocks.push({ t: "table", head, rows });
      continue;
    }
    for (const [re, t] of [[bullet, "ul"], [numbered, "ol"]] as const) {
      if (!re.test(line)) continue;
      const items: Inline[][] = [];
      for (; i < lines.length && re.test(lines[i] ?? ""); i++) items.push(inline((lines[i] ?? "").replace(re, "")));
      blocks.push({ t, items });
      break;
    }
    if (bullet.test(line) || numbered.test(line)) continue;
    const para: string[] = [];
    for (; i < lines.length; i++) {
      const l = lines[i] ?? "";
      if (para.length && (!l.trim() || l.trim().startsWith("```") || heading.test(l) || bullet.test(l) || numbered.test(l) ||
        (l.trim().startsWith("|") && separator.test(lines[i + 1] ?? "")))) break;
      para.push(l);
    }
    blocks.push({ t: "p", c: inline(para.join("\n")) });
  }
  return blocks;
}
