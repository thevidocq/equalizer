import type { ReactNode } from "react";
import { Navigate, useLocation } from "react-router";
import { Notice } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useMe } from "../hooks/useSession";
import { nextStep } from "../pages/steps";

/** Shows its children only to users who completed both login steps. */
export function RequireAuth({ children }: { children: ReactNode }) {
  const me = useMe();
  const location = useLocation();
  if (me.isPending) return null;
  if (me.isError) {
    return (
      <div className="p-8">
        <Notice tone="error">The panel is not answering: {describeError(me.error)}</Notice>
      </div>
    );
  }
  if (!me.data) return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  const step = nextStep(me.data);
  if (step) return <Navigate to={step} replace />;
  return <>{children}</>;
}
