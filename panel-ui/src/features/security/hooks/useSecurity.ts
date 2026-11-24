import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap, type Schemas } from "@/shared/api/client";

export type SecurityOverview = Schemas["SecurityOverview"];
export type SecurityTarget = Schemas["SecurityTarget"];
export type SecurityFinding = Schemas["SecurityFinding"];
export type ScanStart = Schemas["SecurityScanStart"];

/**
 * How often to read the scans again: every 15 seconds while one goes on or
 * its map draft is being built (the first 5 minutes); every minute while
 * the draft waits for the map to be free (a version in review), up to the
 * day Equalizer keeps trying; never otherwise.
 */
export function pollEvery(o: SecurityOverview | undefined, now: number): number | false {
  let every: number | false = false;
  for (const t of o?.targets ?? []) {
    const s = t.scan;
    if (!s) continue;
    if (s.status === "pending" || s.status === "running") return 15_000;
    const since = now - (s.completed_at ? Date.parse(s.completed_at) : 0);
    if (s.status !== "completed" || s.draft_version != null || s.draft_note) continue;
    if (since < 5 * 60_000) return 15_000;
    if (since < 24 * 3600_000) every = 60_000;
  }
  return every;
}

/** A scan going on, or one just ended whose map draft is still being built. */
export function going(o: SecurityOverview | undefined, now: number): boolean {
  return pollEvery(o, now) === 15_000;
}

/** The site's scans and findings, read again as pollEvery says. */
export function useSecurity(app: string) {
  return useQuery({
    queryKey: ["security", app],
    queryFn: () => unwrap(api.GET("/apps/{app}/security", { params: { path: { app } } })),
    refetchInterval: (q) => pollEvery(q.state.data, Date.now()),
  });
}

export function useStartScan(app: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: ScanStart) => unwrap(api.POST("/apps/{app}/security/scans", { params: { path: { app } }, body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["security", app] }),
  });
}
