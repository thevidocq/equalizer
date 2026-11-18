import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

export function useConnectors(app: string) {
  return useQuery({
    queryKey: ["connectors", app],
    queryFn: async () => (await unwrap(api.GET("/apps/{app}/connectors", { params: { path: { app } } }))).items,
  });
}

export function useConnectorActions(app: string) {
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: ["connectors", app] });
  const create = useMutation({
    mutationFn: (id: string) => unwrap(api.POST("/apps/{app}/connectors", { params: { path: { app } }, body: { id } })),
    onSuccess: refresh,
  });
  const rotate = useMutation({
    mutationFn: (id: string) => unwrap(api.POST("/apps/{app}/connectors/{id}/rotate", { params: { path: { app, id } } })),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: (id: string) => unwrap(api.DELETE("/apps/{app}/connectors/{id}", { params: { path: { app, id } } })),
    onSuccess: refresh,
  });
  return { create, rotate, remove };
}
