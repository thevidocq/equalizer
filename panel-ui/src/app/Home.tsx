import { Navigate } from "react-router";
import { useMe } from "@/features/auth";

/** Platform users start from the tenants, tenant users from their sites. */
export function Home() {
  const me = useMe().data;
  if (!me) return null;
  return <Navigate to={me.platform_role ? "/tenants" : `/t/${me.tenant}`} replace />;
}
