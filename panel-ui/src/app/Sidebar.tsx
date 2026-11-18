import { useState } from "react";
import { NavLink, useNavigate } from "react-router";
import { Wordmark, canManage, useLogout, useMe } from "@/features/auth";
import { CreateSiteDialog, SiteList } from "@/features/sites";

const link = ({ isActive }: { isActive: boolean }) =>
  `block rounded-[var(--radius-control)] px-3 py-1.5 text-sm ${isActive ? "bg-paper/12 text-paper" : "text-paper/75 hover:text-paper"}`;

/** Platform users see the customer list; tenant people see their own tenant. Never both. */
export function Sidebar() {
  const me = useMe().data;
  const logout = useLogout();
  const navigate = useNavigate();
  const [adding, setAdding] = useState(false);
  const tenant = me?.tenant ?? "";

  return (
    <aside className="flex w-64 shrink-0 flex-col gap-6 bg-ink px-3 py-5 text-paper">
      <div className="px-3"><Wordmark inverted /></div>
      {tenant ? (
        <nav aria-label="Protected sites" className="flex flex-col gap-1">
          <NavLink to={`/t/${tenant}`} end className={link}>Protected sites</NavLink>
          <div className="ml-2 border-l border-paper/15 pl-1"><SiteList tenant={tenant} /></div>
          {canManage(me, tenant) ? (
            <button type="button" onClick={() => setAdding(true)} className="px-3 py-1.5 text-left text-sm text-paper/75 hover:text-paper">
              + Protect a site
            </button>
          ) : null}
        </nav>
      ) : null}
      <nav aria-label="Administration" className="flex flex-col gap-1">
        {tenant && canManage(me, tenant) ? <NavLink to={`/t/${tenant}/connectors`} className={link}>Connectors</NavLink> : null}
        {tenant ? <NavLink to={`/t/${tenant}/people`} className={link}>People</NavLink> : null}
        {tenant ? <NavLink to={`/t/${tenant}/activity`} className={link}>Activity</NavLink> : null}
        {me?.platform_role ? <NavLink to="/tenants" className={link}>Tenants</NavLink> : null}
        {me?.platform_role ? <NavLink to="/activity" end className={link}>Activity</NavLink> : null}
      </nav>
      <div className="mt-auto flex flex-col gap-1 border-t border-paper/15 px-3 pt-4 text-sm">
        <NavLink to="/account" className="truncate text-paper/75 hover:text-paper" title="Your account">{me?.email}</NavLink>
        <button type="button" className="text-left text-paper/75 hover:text-paper" onClick={() => logout.mutate(undefined, { onSettled: () => navigate("/login") })}>
          Log out
        </button>
      </div>
      {tenant ? <CreateSiteDialog tenant={tenant} open={adding} onOpenChange={setAdding} /> : null}
    </aside>
  );
}
