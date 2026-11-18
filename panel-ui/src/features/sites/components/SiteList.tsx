import { NavLink } from "react-router";
import { useApps } from "../hooks/useSites";
import { SiteStatus } from "./SiteStatus";

/** The sidebar list of the tenant's protected sites. */
export function SiteList({ tenant }: { tenant: string }) {
  const apps = useApps(tenant);
  if (!apps.data) return null;
  if (apps.data.length === 0) return <p className="px-3 text-sm text-paper/60">No sites yet.</p>;
  return (
    <ul className="flex flex-col">
      {apps.data.map((a) => (
        <li key={a.id}>
          <NavLink
            to={`/t/${tenant}/sites/${a.id}`}
            className={({ isActive }) =>
              `flex items-center gap-2.5 rounded-[var(--radius-control)] px-3 py-1.5 text-sm ${
                isActive ? "bg-paper/12 text-paper" : "text-paper/75 hover:text-paper"
              }`
            }
          >
            <SiteStatus app={a} />
            <span className="truncate">{a.name}</span>
          </NavLink>
        </li>
      ))}
    </ul>
  );
}
