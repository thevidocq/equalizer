import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap, type Schemas } from "@/shared/api/client";

export type Discovery = Schemas["Discovery"];
const active = (d?: Discovery[]) => d?.some((x) => x.status === "queued" || x.status === "running");

export function useDiscoveries(app: string) {
  return useQuery({
    queryKey: ["discoveries", app],
    queryFn: async () => (await unwrap(api.GET("/apps/{app}/discoveries", { params: { path: { app } } }))).items,
    refetchInterval: (q) => (active(q.state.data) ? 4000 : false),
  });
}

/** How often to read a discovery again: every 4 s while it runs; every 15 s
 * while its security study waits for the check of the draft; never after. */
export function discoveryPoll(d?: Discovery): number | false {
  if (!d) return false;
  if (d.status === "queued" || d.status === "running") return 4000;
  const sec = (d.report as { security?: { pending?: boolean } } | null)?.security;
  return d.status === "done" && sec?.pending ? 15000 : false;
}

export function useDiscovery(app: string, id: number | null) {
  return useQuery({
    queryKey: ["discovery", app, id],
    queryFn: () => unwrap(api.GET("/apps/{app}/discoveries/{id}", { params: { path: { app, id: String(id) } } })),
    enabled: id != null,
    refetchInterval: (q) => discoveryPoll(q.state.data),
  });
}

export function useStartDiscovery(app: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: Schemas["DiscoveryStart"]) => unwrap(api.POST("/apps/{app}/discoveries", { params: { path: { app } }, body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["discoveries", app] }),
  });
}

export type Frequency = "off" | "daily" | "weekly";
export type ScheduleChange = { discovery_schedule: Frequency; discovery_hour: number; discovery_weekday: number };

/** Continuous discovery: how often, at which hour and on which day, in the person's time zone. */
export function useSchedule(app: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: ScheduleChange) => unwrap(api.PATCH("/apps/{app}", { params: { path: { app } }, body })),
    onSuccess: (a) => qc.setQueryData(["app", app], a),
  });
}
