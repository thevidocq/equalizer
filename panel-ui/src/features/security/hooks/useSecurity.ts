import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap, type Schemas } from "@/shared/api/client";

export type SecurityOverview = Schemas["SecurityOverview"];
export type SecurityTarget = Schemas["SecurityTarget"];
export type SecurityFinding = Schemas["SecurityFinding"];
export type ScanStart = Schemas["SecurityScanStart"];

const going = (o?: SecurityOverview) =>
  !!o?.targets.some((t) => t.scan?.status === "pending" || t.scan?.status === "running");

/** The site's scans and findings; refreshed every 15 seconds while a scan goes on. */
export function useSecurity(app: string) {
  return useQuery({
    queryKey: ["security", app],
    queryFn: () => unwrap(api.GET("/apps/{app}/security", { params: { path: { app } } })),
    refetchInterval: (q) => (going(q.state.data) ? 15_000 : false),
  });
}

export function useStartScan(app: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: ScanStart) => unwrap(api.POST("/apps/{app}/security/scans", { params: { path: { app } }, body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["security", app] }),
  });
}
