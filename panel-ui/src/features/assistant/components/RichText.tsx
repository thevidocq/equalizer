import type { ReactNode } from "react";

/** `code` spans of a line. */
function inline(line: string): ReactNode[] {
  return line.split(/(`[^`]+`)/g).map((part, i) =>
    part.startsWith("`") && part.endsWith("`") && part.length > 2
      ? <code key={i} className="rounded bg-paper px-1 py-0.5 font-mono text-[0.8125rem] break-all">{part.slice(1, -1)}</code>
      : part.replace(/\*\*/g, ""),
  );
}

/**
 * The model's plain text: paragraphs, "- " bullets, `code`. Rendered as
 * text, never as HTML: an answer can quote a path an attacker chose.
 */
export function RichText({ text }: { text: string }) {
  const blocks = text.trim().split(/\n\s*\n/);
  return (
    <div className="flex flex-col gap-2">
      {blocks.map((block, i) => {
        const lines = block.split("\n");
        if (lines.every((l) => /^\s*[-*•]\s+/.test(l))) {
          return (
            <ul key={i} className="ml-4 list-disc space-y-1">
              {lines.map((l, j) => <li key={j}>{inline(l.replace(/^\s*[-*•]\s+/, ""))}</li>)}
            </ul>
          );
        }
        return <p key={i}>{lines.flatMap((l, j) => (j ? [<br key={`b${j}`} />, ...inline(l)] : inline(l)))}</p>;
      })}
    </div>
  );
}
