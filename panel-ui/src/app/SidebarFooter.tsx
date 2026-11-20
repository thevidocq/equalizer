import { Link, NavLink, useNavigate } from "react-router";
import { useLogout } from "@/features/auth";

type Props = { name?: string; email?: string; role?: string | null; platform?: boolean };

const roleNames: Record<string, string> = {
  admin: "Administrator", viewer: "Viewer", operator: "Operator",
};

function initials(name?: string, email?: string): string {
  const words = (name || email || "?").replace(/@.*/, "").split(/[\s._-]+/).filter(Boolean);
  return ((words[0]?.[0] ?? "?") + (words[1]?.[0] ?? "")).toUpperCase();
}

const icon = "inline-flex size-7 items-center justify-center rounded-md transition-colors";

/** The account card: who is logged in, with Help and Log out at hand. */
export function SidebarFooter({ name, email, role, platform }: Props) {
  const roleName = platform ? (role === "admin" ? "Platform admin" : "Platform operator") : role ? roleNames[role] ?? role : "";
  const logout = useLogout();
  const navigate = useNavigate();
  return (
    <div className="mt-auto border-t border-paper/15 pt-3">
      <div className="flex items-center gap-2.5 rounded-[var(--radius-control)] px-2 py-1.5">
        <Link to="/account" title="Your account" className="group flex min-w-0 flex-1 items-center gap-2.5">
          <span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-paper/12 text-[12px] font-semibold text-paper">
            {initials(name, email)}
          </span>
          <span className="flex min-w-0 flex-col leading-tight">
            <span className="truncate text-[13px] font-medium text-paper group-hover:underline">{name || email}</span>
            <span className="truncate text-[11.5px] text-paper/55">{roleName || email}</span>
          </span>
        </Link>
        <div className="flex shrink-0 items-center gap-0.5">
          <NavLink
            to="/help" aria-label="Help" title="Help"
            className={({ isActive }) => `${icon} ${isActive ? "bg-paper/12 text-paper" : "text-paper/60 hover:bg-paper/10 hover:text-paper"}`}
          >
            <svg viewBox="0 0 24 24" className="size-4" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <circle cx="12" cy="12" r="9" />
              <path d="M9.6 9.3a2.5 2.5 0 0 1 4.8.9c0 1.7-2.4 2.2-2.4 3.6" />
              <circle cx="12" cy="17" r="0.6" fill="currentColor" />
            </svg>
          </NavLink>
          <button
            type="button" aria-label="Log out" title="Log out" disabled={logout.isPending}
            className={`${icon} text-[#d9a0a0] hover:bg-[#d9a0a0]/12 hover:text-[#ebb5b5]`}
            onClick={() => logout.mutate(undefined, { onSettled: () => navigate("/login") })}
          >
            <svg viewBox="0 0 24 24" className="size-4" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M10 4H6.5A1.5 1.5 0 0 0 5 5.5v13A1.5 1.5 0 0 0 6.5 20H10" />
              <path d="M15 8l4 4-4 4" />
              <path d="M19 12H10" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  );
}
