import { useState } from "react";
import { NavLink, useNavigate, useParams } from "react-router";
import { Wordmark, canManage, useLogout, useMe } from "@/features/auth";
import { CreateSiteDialog, SiteList } from "@/features/sites";
import { useTenants } from "@/features/tenants";

const link = ({ isActive }: { isActive: boolean }) =>
  `block rounded-[var(--radius-control)] px-3 py-1.5 text-sm ${isActive ? "bg-paper/12 text-paper" : "text-paper/75 hover:text-paper"}`;

export function Sidebar() {
  const tenant = useParams().tenant ?? "";
  const me = useMe().data;
  const tenants = useTenants();
  const logout = useLogout();
  const navigate = useNavigate();
  const [adding, setAdding] = useState(false);
  const platform = !!me?.platform_role;

  return (
    <aside className="flex w-64 shrink-0 flex-col gap-6 bg-ink px-3 py-5 text-paper">
      <div className="px-3"><Wordmark inverted /></div>
      {platform ? (
        <div className="flex flex-col gap-1 px-3">
          <label htmlFor="tenant-pick" className="text-xs text-paper/60">Tenant</label>
          <select id="tenant-pick" value={tenant} onChange={(e) => navigate(e.target.value ? `/t/${e.target.value}` : "/tenants")}
            className="rounded-[var(--radius-control)] border border-paper/25 bg-ink px-2 py-1.5 text-sm text-paper">
            <option value="">All tenants</option>
            {tenants.data?.map((t) => <option key={t.id} value={t.id}>{t.name}</option>)}
          </select>
        </div>
      ) : null}
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
        {tenant ? <NavLink to={`/t/${tenant}/people`} className={link}>People</NavLink> : null}
        {tenant ? <NavLink to={`/t/${tenant}/activity`} className={link}>Activity</NavLink> : null}
        {platform ? <NavLink to="/tenants" className={link}>Tenants</NavLink> : null}
        {platform ? <NavLink to="/activity" end className={link}>All activity</NavLink> : null}
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
