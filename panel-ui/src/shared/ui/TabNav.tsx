import { NavLink } from "react-router";

export type Tab = { to: string; label: string };

/**
 * Tabs that are routes: each tab has its own URL. On a narrow screen the row
 * scrolls sideways on its own; the page never gets wider than the screen.
 */
export function TabNav({ tabs, label }: { tabs: Tab[]; label: string }) {
  return (
    <nav aria-label={label} className="flex max-w-full gap-6 overflow-x-auto shadow-[inset_0_-1px_0_var(--color-rule)] [scrollbar-width:none]">
      {tabs.map((t) => (
        <NavLink
          key={t.to}
          to={t.to}
          className={({ isActive }) =>
            `shrink-0 whitespace-nowrap border-b-2 pb-2.5 text-sm font-semibold ${
              isActive ? "border-ink text-ink" : "border-transparent text-ink-soft hover:text-ink"
            }`
          }
        >
          {t.label}
        </NavLink>
      ))}
    </nav>
  );
}
