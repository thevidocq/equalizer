import { useQuery } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

const every = 30_000;

/** Health of the tenant's sites, refreshed every 30 seconds. */
export function useTenantStatus(tenant: string) {
  return useQuery({
    queryKey: ["status", tenant],
    queryFn: () => unwrap(api.GET("/tenants/{tenant}/status", { params: { path: { tenant } } })),
    refetchInterval: every,
    enabled: !!tenant,
  });
}

/** Health of the platform's services (platform users). */
export function usePlatformStatus() {
  return useQuery({
    queryKey: ["status", "platform"],
    queryFn: () => unwrap(api.GET("/status/platform")),
    refetchInterval: every,
  });
}
