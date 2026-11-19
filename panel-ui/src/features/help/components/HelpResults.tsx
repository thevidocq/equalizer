import { Link } from "react-router";
import type { Hit } from "../guide";

/** Search results: topic, where it is, and the text around the match. */
export function HelpResults({ query, hits, onOpen }: { query: string; hits: Hit[]; onOpen: () => void }) {
  if (hits.length === 0) {
    return <p className="text-ink-soft">Nothing in the guide matches "{query}". Try fewer or other words.</p>;
  }
  return (
    <section aria-label="Search results" className="flex flex-col gap-1">
      <p className="mb-2 text-sm text-ink-soft">{hits.length === 1 ? "1 topic" : `${hits.length} topics`} for "{query}"</p>
      <ul className="flex flex-col divide-y divide-rule-soft">
        {hits.map(({ topic, snippet }) => (
          <li key={topic.id} className="py-3">
            <Link to={`/help/${topic.id}`} onClick={onOpen} className="font-semibold text-signal hover:underline">{topic.title}</Link>
            <span className="ml-2 text-xs text-ink-soft">{topic.macroTitle}</span>
            <p className="mt-1 max-w-prose text-sm text-ink-soft">{snippet}</p>
          </li>
        ))}
      </ul>
    </section>
  );
}
