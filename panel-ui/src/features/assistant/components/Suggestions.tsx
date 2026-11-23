const examples = [
  "Is everything working on this site?",
  "Did real visitors get refused today?",
  "Which CMS, plugins and versions does the site run?",
  "What changed in the latest version of the map?",
  "Why was the last request to /shop/ blocked?",
  "How do I check the pages behind the login?",
];

/** What to ask, before the first question. */
export function Suggestions({ onPick }: { onPick: (q: string) => void }) {
  return (
    <div className="flex flex-col gap-3 text-sm text-ink-soft">
      <p>
        Ask about this site: whether it works, what the protection does, what happened, how to do something in the
        panel. The assistant reads the site's status, map, versions, checks, discovery, logs and history, and the
        guide; it can propose a change to the map, which only an administrator turns into a draft.
      </p>
      <ul className="flex flex-col gap-2">
        {examples.map((q) => (
          <li key={q}>
            <button type="button" onClick={() => onPick(q)}
              className="w-full rounded-[var(--radius-control)] border border-rule bg-surface px-3 py-2 text-left text-ink hover:border-signal">
              {q}
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
