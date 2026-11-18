import { useQuery } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

/** The caller's own activity: the tenant's, or the platform's for platform users. */
export function useAudit() {
  return useQuery({
    queryKey: ["audit"],
    queryFn: async () => (await unwrap(api.GET("/audit"))).items,
  });
}
