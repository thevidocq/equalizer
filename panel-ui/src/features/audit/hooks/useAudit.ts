import { useInfiniteQuery } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

export type AuditFilter = { q: string; from?: string; to?: string };

/** The caller's own activity, newest first, 200 per page: the tenant's, or the platform's for platform users. */
export function useAudit(f: AuditFilter) {
  return useInfiniteQuery({
    queryKey: ["audit", f],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      unwrap(api.GET("/audit", { params: { query: { q: f.q || undefined, from: f.from, to: f.to, cursor: pageParam } } })),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
}
