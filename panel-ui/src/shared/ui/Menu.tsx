import { useEffect, useId, useRef, useState, type KeyboardEvent, type ReactNode } from "react";

export type MenuItem = { label: string; note?: string; onSelect: () => void };

/**
 * A button that opens a short list of actions below it. Arrows move between
 * them, Enter chooses, Escape or a click elsewhere closes the list.
 */
export function Menu({ label, items }: { label: ReactNode; items: MenuItem[] }) {
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const root = useRef<HTMLDivElement>(null);
  const button = useRef<HTMLButtonElement>(null);
  const list = useRef<HTMLUListElement>(null);
  const id = useId();

  useEffect(() => {
    if (!open) return;
    const away = (e: MouseEvent) => { if (!root.current?.contains(e.target as Node)) setOpen(false); };
    document.addEventListener("mousedown", away);
    return () => document.removeEventListener("mousedown", away);
  }, [open]);
  useEffect(() => {
    if (open) (list.current?.children[active]?.firstElementChild as HTMLElement | null)?.focus();
  }, [open, active]);

  const close = () => { setOpen(false); button.current?.focus(); };
  const choose = (i: number) => { close(); items[i]?.onSelect(); };
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Escape") { e.preventDefault(); close(); }
    else if (e.key === "ArrowDown") { e.preventDefault(); setActive((a) => (a + 1) % items.length); }
    else if (e.key === "ArrowUp") { e.preventDefault(); setActive((a) => (a - 1 + items.length) % items.length); }
    else if (e.key === "Home") { e.preventDefault(); setActive(0); }
    else if (e.key === "End") { e.preventDefault(); setActive(items.length - 1); }
    else if (e.key === "Tab") setOpen(false);
  };

  return (
    <div ref={root} className="relative inline-block" onKeyDown={open ? onKey : undefined}>
      <button
        ref={button} type="button" aria-haspopup="menu" aria-expanded={open} aria-controls={open ? id : undefined}
        onClick={() => { setActive(0); setOpen((o) => !o); }}
        onKeyDown={(e) => { if (!open && (e.key === "ArrowDown" || e.key === "Enter" || e.key === " ")) { e.preventDefault(); setActive(0); setOpen(true); } }}
        className="inline-flex items-center gap-2 rounded-[var(--radius-control)] border border-rule bg-surface px-3.5 py-2 text-sm font-semibold text-ink hover:border-ink-soft"
      >
        {label}
        <svg viewBox="0 0 24 24" className={`size-4 text-ink-soft transition-transform ${open ? "rotate-180" : ""}`} fill="none" stroke="currentColor"
          strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M6 9l6 6 6-6" /></svg>
      </button>
      {open ? (
        <ul ref={list} id={id} role="menu" className="absolute left-0 z-20 mt-1 min-w-60 rounded-[var(--radius-control)] border border-rule bg-surface py-1 shadow-md">
          {items.map((it, i) => (
            <li key={it.label} role="none">
              <button
                type="button" role="menuitem" tabIndex={i === active ? 0 : -1}
                onClick={() => choose(i)} onMouseEnter={() => setActive(i)}
                className="flex w-full flex-col items-start px-3 py-2 text-left text-sm hover:bg-paper focus:bg-paper focus:outline-none"
              >
                <span className="font-semibold">{it.label}</span>
                {it.note ? <span className="text-xs text-ink-soft">{it.note}</span> : null}
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
