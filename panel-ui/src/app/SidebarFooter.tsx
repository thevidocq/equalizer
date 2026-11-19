import { NavLink, useNavigate } from "react-router";
import { useLogout } from "@/features/auth";

const iconButton = "inline-flex size-8 items-center justify-center rounded-[var(--radius-control)] transition-colors";

/** Account on the left; help and logout as icons on the right. */
export function SidebarFooter({ email }: { email?: string }) {
  const logout = useLogout();
  const navigate = useNavigate();
  return (
    <div className="mt-auto flex items-center gap-2 border-t border-paper/15 px-3 pt-4 text-sm">
      <NavLink to="/account" className="min-w-0 flex-1 truncate text-paper/75 hover:text-paper" title="Your account">{email}</NavLink>
      <NavLink
        to="/help" aria-label="Help" title="Help"
        className={({ isActive }) => `${iconButton} ${isActive ? "bg-paper/10 text-paper" : "text-paper/70 hover:bg-paper/10 hover:text-paper"}`}
      >
        <svg viewBox="0 0 24 24" className="size-[18px]" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <circle cx="12" cy="12" r="9" />
          <path d="M9.6 9.3a2.5 2.5 0 0 1 4.8.9c0 1.7-2.4 2.2-2.4 3.6" />
          <circle cx="12" cy="17" r="0.6" fill="currentColor" />
        </svg>
      </NavLink>
      <button
        type="button" aria-label="Log out" title="Log out" disabled={logout.isPending}
        className={`${iconButton} text-[#e6a3a3] hover:bg-[#e6a3a3]/12 hover:text-[#f0b8b8]`}
        onClick={() => logout.mutate(undefined, { onSettled: () => navigate("/login") })}
      >
        <svg viewBox="0 0 24 24" className="size-[18px]" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <path d="M10 4H6.5A1.5 1.5 0 0 0 5 5.5v13A1.5 1.5 0 0 0 6.5 20H10" />
          <path d="M15 8l4 4-4 4" />
          <path d="M19 12H10" />
        </svg>
      </button>
    </div>
  );
}
