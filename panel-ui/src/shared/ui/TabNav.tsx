import { NavLink } from "react-router";

export type Tab = { to: string; label: string };

/** Tabs that are routes: each tab has its own URL. */
export function TabNav({ tabs, label }: { tabs: Tab[]; label: string }) {
  return (
    <nav aria-label={label} className="flex gap-6 border-b border-rule">
      {tabs.map((t) => (
        <NavLink
          key={t.to}
          to={t.to}
          className={({ isActive }) =>
            `-mb-px border-b-2 pb-2.5 text-sm font-semibold ${
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
