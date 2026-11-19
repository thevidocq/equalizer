import { useMemo } from "react";
import { useMe } from "@/features/auth";
import { parseGuide, type Guide } from "../guide";
import tenantGuide from "../content/tenant.md?raw";
import adminGuide from "../content/admin.md?raw";

/** The guide for whoever is logged in: the platform's or the customer's. */
export function useGuide(): Guide | null {
  const me = useMe().data;
  const platform = Boolean(me?.platform_role);
  return useMemo(() => (me ? parseGuide(platform ? adminGuide : tenantGuide) : null), [me, platform]);
}
