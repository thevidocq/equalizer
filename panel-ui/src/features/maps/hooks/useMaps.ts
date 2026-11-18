import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

export type MapAction = "submit" | "approve" | "reject" | "publish";

export function useMapVersions(app: string) {
  return useQuery({
    queryKey: ["maps", app],
    queryFn: async () => (await unwrap(api.GET("/apps/{app}/maps", { params: { path: { app } } }))).items,
  });
}

export function useMapVersion(app: string, version: number) {
  return useQuery({
    queryKey: ["map", app, version],
    queryFn: () => unwrap(api.GET("/apps/{app}/maps/{version}", { params: { path: { app, version } } })),
    enabled: version > 0,
  });
}

export function useCreateDraft(app: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { document: Record<string, unknown>; note: string }) =>
      unwrap(api.POST("/apps/{app}/maps", { params: { path: { app } }, body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["maps", app] }),
  });
}

export function useMapAction(app: string, version: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (action: MapAction) =>
      unwrap(api.POST("/apps/{app}/maps/{version}/{action}", { params: { path: { app, version, action } } })),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["maps", app] });
      qc.invalidateQueries({ queryKey: ["map", app, version] });
      qc.invalidateQueries({ queryKey: ["app", app] });
      qc.invalidateQueries({ queryKey: ["apps"] });
    },
  });
}
