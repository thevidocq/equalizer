import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap, type Schemas } from "@/shared/api/client";

export type Verification = Schemas["Verification"];

/** The agent's latest verification of a version; polled while it runs. */
export function useVerification(app: string, version: number) {
  const path = { params: { path: { app, version } } };
  return useQuery({
    queryKey: ["verification", app, version],
    queryFn: () => unwrap(api.GET("/apps/{app}/maps/{version}/verify", path)),
    retry: false,
    refetchInterval: (q) => (q.state.data && ["queued", "running"].includes(q.state.data.status) ? 4000 : false),
  });
}

export function useVerify(app: string, version: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.POST("/apps/{app}/maps/{version}/verify", { params: { path: { app, version } } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["verification", app, version] }),
  });
}
