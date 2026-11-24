import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap, type Schemas } from "@/shared/api/client";

export type SecurityOverview = Schemas["SecurityOverview"];
export type SecurityTarget = Schemas["SecurityTarget"];
export type SecurityFinding = Schemas["SecurityFinding"];
export type ScanStart = Schemas["SecurityScanStart"];

/** A scan going on, or one just ended whose map draft is still being built. */
export function going(o: SecurityOverview | undefined, now: number): boolean {
  return !!o?.targets.some((t) => {
    const s = t.scan;
    if (!s) return false;
    if (s.status === "pending" || s.status === "running") return true;
    const ended = s.completed_at ? Date.parse(s.completed_at) : 0;
    return s.status === "completed" && s.draft_version == null && !s.draft_note && now - ended < 5 * 60_000;
  });
}

/** The site's scans and findings; refreshed every 15 seconds while a scan goes on. */
export function useSecurity(app: string) {
  return useQuery({
    queryKey: ["security", app],
    queryFn: () => unwrap(api.GET("/apps/{app}/security", { params: { path: { app } } })),
    refetchInterval: (q) => (going(q.state.data, Date.now()) ? 15_000 : false),
  });
}

export function useStartScan(app: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: ScanStart) => unwrap(api.POST("/apps/{app}/security/scans", { params: { path: { app } }, body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["security", app] }),
  });
}
