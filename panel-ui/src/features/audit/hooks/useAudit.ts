import { useQuery } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

export function useAudit(tenant?: string) {
  return useQuery({
    queryKey: ["audit", tenant ?? ""],
    queryFn: async () => (await unwrap(api.GET("/audit", { params: { query: tenant ? { tenant } : {} } }))).items,
  });
}
