import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";
import type { EventType, Since } from "../components/kinds";

/** from/to: one bar of the refused chart; absent: the whole period. */
export type LogFilter = { app: string; types: EventType[]; q: string; since: Since; from?: string; to?: string };

function query(f: LogFilter, withTypes: boolean) {
  return {
    app: f.app || undefined,
    q: f.q || undefined,
    since: f.since,
    types: withTypes && f.types.length ? f.types.join(",") : undefined,
    // the window narrows the events, never the counts and the chart of the period
    from: withTypes ? f.from : undefined,
    to: withTypes ? f.to : undefined,
  };
}

const live = 5_000;

/** Events of the tenant's sites, newest first, 50 per page; refreshed while live. */
export function useLogs(tenant: string, f: LogFilter, isLive: boolean) {
  return useInfiniteQuery({
    queryKey: ["logs", tenant, f],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      unwrap(api.GET("/tenants/{tenant}/logs", { params: { path: { tenant }, query: { ...query(f, true), before: pageParam } } })),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
    refetchInterval: isLive ? live : false,
    enabled: f.types.length > 0,
  });
}

/** Back to the newest page only: live refresh of several pages would leave gaps. */
export function useRestartLogs(tenant: string) {
  const qc = useQueryClient();
  return () => void qc.resetQueries({ queryKey: ["logs", tenant] });
}

/** Counts per type and the refused chart, for the same filters. */
export function useLogStats(tenant: string, f: LogFilter, isLive: boolean) {
  return useQuery({
    queryKey: ["log-stats", tenant, f.app, f.q, f.since],
    queryFn: () => unwrap(api.GET("/tenants/{tenant}/logs/stats", { params: { path: { tenant }, query: query(f, false) } })),
    refetchInterval: isLive ? live * 6 : false,
  });
}

/** Downloads the filtered events as CSV (the request carries the panel header). */
export function useLogExport(tenant: string) {
  return useMutation({
    mutationFn: async (f: LogFilter) => {
      const res = await api.GET("/tenants/{tenant}/logs/export", {
        params: { path: { tenant }, query: query(f, true) },
        parseAs: "blob",
      });
      if (!res.response.ok || !res.data) throw new Error("The export failed. Try again in a moment.");
      const url = URL.createObjectURL(res.data as Blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `equalizer-logs-${new Date().toISOString().slice(0, 10)}.csv`;
      a.click();
      URL.revokeObjectURL(url);
    },
  });
}
