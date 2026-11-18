import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap, type Schemas } from "@/shared/api/client";

export type MapReview = Schemas["MapReview"];

export function useReview(app: string, version: number) {
  const path = { params: { path: { app, version } } };
  return useQuery({
    queryKey: ["review", app, version],
    queryFn: () => unwrap(api.GET("/apps/{app}/maps/{version}/review", path)),
    refetchInterval: (q) => (q.state.data && ["queued", "running"].includes(q.state.data.ai.status) ? 4000 : false),
  });
}

export function useAskReview(app: string, version: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.POST("/apps/{app}/maps/{version}/review", { params: { path: { app, version } } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["review", app, version] }),
  });
}
