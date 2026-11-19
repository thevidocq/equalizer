import { NavLink } from "react-router";
import type { Guide } from "../guide";

const link = ({ isActive }: { isActive: boolean }) =>
  `block rounded-[var(--radius-control)] px-2 py-1 text-sm ${isActive ? "bg-signal-soft font-medium text-signal" : "text-ink-soft hover:text-ink"}`;

/** Macro topics with their topics. */
export function HelpNav({ guide }: { guide: Guide }) {
  return (
    <nav aria-label="Help topics" className="flex flex-col gap-4">
      {guide.macros.map((m) => (
        <div key={m.id}>
          <p className="px-2 pb-1 text-[13px] font-semibold text-ink">{m.title}</p>
          <ul className="flex flex-col gap-0.5 border-l border-rule pl-2">
            {m.topics.map((t) => (
              <li key={t.id}><NavLink to={`/help/${t.id}`} className={link}>{t.title}</NavLink></li>
            ))}
          </ul>
        </div>
      ))}
    </nav>
  );
}
