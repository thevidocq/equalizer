import { useState } from "react";
import { NavLink } from "react-router";
import { canManage, hasRole, useMe } from "@/features/auth";
import { CreateSiteDialog, SiteList } from "@/features/sites";
import { Logo } from "@/shared/ui";
import { SidebarFooter } from "./SidebarFooter";

const link = ({ isActive }: { isActive: boolean }) =>
  `block rounded-[var(--radius-control)] px-3 py-1.5 text-sm ${isActive ? "bg-paper/12 text-paper" : "text-paper/75 hover:text-paper"}`;

/** Platform users see the customer list; tenant people see their own tenant. Never both. */
export function Sidebar() {
  const me = useMe().data;
  const [adding, setAdding] = useState(false);
  const tenant = me?.tenant ?? "";

  return (
    <aside className="flex h-full flex-col gap-6 overflow-y-auto bg-ink px-3 py-5 text-paper">
      <div className="px-3"><Logo inverted /></div>
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
        {tenant ? <NavLink to={`/t/${tenant}/status`} className={link}>Status</NavLink> : null}
        {tenant && canManage(me, tenant) ? <NavLink to={`/t/${tenant}/connectors`} className={link}>Connectors</NavLink> : null}
        {tenant ? <NavLink to={`/t/${tenant}/people`} className={link}>People</NavLink> : null}
        {tenant ? <NavLink to={`/t/${tenant}/activity`} className={link}>Activity</NavLink> : null}
        {tenant && hasRole(me, tenant, "admin", "approver", "auditor") ? <NavLink to={`/t/${tenant}/logs`} className={link}>Logs</NavLink> : null}
        {tenant && canManage(me, tenant) ? <NavLink to={`/t/${tenant}/settings`} className={link}>Settings</NavLink> : null}
        {me?.platform_role ? <NavLink to="/tenants" className={link}>Tenants</NavLink> : null}
        {me?.platform_role ? <NavLink to="/status" className={link}>Platform status</NavLink> : null}
        {me?.platform_role ? <NavLink to="/activity" end className={link}>Activity</NavLink> : null}
      </nav>
      <SidebarFooter name={me?.name} email={me?.email} role={me?.tenant_role ?? me?.platform_role} platform={!me?.tenant_role && !!me?.platform_role} />
      {tenant ? <CreateSiteDialog tenant={tenant} open={adding} onOpenChange={setAdding} /> : null}
    </aside>
  );
}
