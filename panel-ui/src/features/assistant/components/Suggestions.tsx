const examples = [
  "Why was the last request to /shop/ blocked?",
  "Did real visitors get refused today?",
  "What does the map check on the home page?",
  "What did the last discovery leave open?",
  "Who published the current map, and when?",
];

/** What to ask, before the first question. */
export function Suggestions({ onPick }: { onPick: (q: string) => void }) {
  return (
    <div className="flex flex-col gap-3 text-sm text-ink-soft">
      <p>
        Ask about this site: its map, its last discovery, its logs and its history. The assistant reads them and
        answers; it can propose a change to the map, which only an administrator turns into a draft.
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
