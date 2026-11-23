import { useEffect, useState } from "react";
import { Outlet, useLocation } from "react-router";
import { Logo, setTimeZone } from "@/shared/ui";
import { useMe } from "@/features/auth";
import { CreateSiteDialog } from "@/features/sites";
import { Sidebar } from "./Sidebar";

/**
 * The sidebar stays on the left on wide screens. On phones and narrow
 * windows it becomes a menu opened from a top bar, so pages get the whole
 * width.
 *
 * The "Protect a site" dialog is owned here, not inside the sidebar: the
 * mobile sidebar is unmounted when the menu closes, so a dialog rendered
 * inside it would disappear the moment the menu closes behind it.
 */
export function Shell() {
  const [open, setOpen] = useState(false);
  const [adding, setAdding] = useState(false);
  const me = useMe().data;
  const tenant = me?.tenant ?? "";
  setTimeZone(me?.time_zone ?? "UTC"); // before the pages render their dates
  const { pathname } = useLocation();
  useEffect(() => { setOpen(false); }, [pathname]);
  useEffect(() => {
    if (!open) return;
    const close = (e: KeyboardEvent) => { if (e.key === "Escape") setOpen(false); };
    window.addEventListener("keydown", close);
    return () => window.removeEventListener("keydown", close);
  }, [open]);

  const openAdd = () => { setOpen(false); setAdding(true); }; // close the mobile menu, then open the dialog above it

  return (
    <div className="min-h-screen lg:flex">
      <div className="sticky top-0 hidden h-screen w-64 shrink-0 lg:block"><Sidebar onAddSite={openAdd} /></div>
      <header className="sticky top-0 z-30 flex items-center justify-between bg-ink px-4 py-3 lg:hidden">
        <Logo inverted tagline={false} />
        <button
          type="button" onClick={() => setOpen(true)} aria-expanded={open} aria-controls="mobile-menu"
          className="rounded-[var(--radius-control)] px-3 py-1.5 text-sm font-medium text-paper hover:bg-paper/10"
        >
          Menu
        </button>
      </header>
      {open ? (
        <div className="fixed inset-0 z-40 lg:hidden">
          <button type="button" aria-label="Close the menu" onClick={() => setOpen(false)} className="absolute inset-0 bg-ink/50" />
          <div id="mobile-menu" role="dialog" aria-modal="true" aria-label="Menu" className="absolute inset-y-0 left-0 w-72 max-w-[85vw]">
            <Sidebar onAddSite={openAdd} />
          </div>
        </div>
      ) : null}
      <main key={me?.time_zone ?? "UTC"} className="min-w-0 flex-1 px-4 py-6 sm:px-6 lg:mr-[var(--assistant-width,0px)] lg:px-10 lg:py-8">
        <div className="mx-auto max-w-6xl">
          <Outlet />
        </div>
      </main>
      {tenant ? <CreateSiteDialog tenant={tenant} open={adding} onOpenChange={setAdding} /> : null}
    </div>
  );
}
