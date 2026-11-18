import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

/** Connectors of the tenant: one per server, shared by its sites. */
export function useConnectors(tenant: string) {
  return useQuery({
    queryKey: ["connectors", tenant],
    queryFn: async () => (await unwrap(api.GET("/tenants/{tenant}/connectors", { params: { path: { tenant } } }))).items,
  });
}

export function useConnectorActions(tenant: string) {
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: ["connectors", tenant] });
  const path = (id: string) => ({ params: { path: { tenant, id } } });
  const create = useMutation({
    mutationFn: (id: string) => unwrap(api.POST("/tenants/{tenant}/connectors", { params: { path: { tenant } }, body: { id } })),
    onSuccess: refresh,
  });
  const rotate = useMutation({
    mutationFn: (id: string) => unwrap(api.POST("/tenants/{tenant}/connectors/{id}/rotate", path(id))),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: (id: string) => unwrap(api.DELETE("/tenants/{tenant}/connectors/{id}", path(id))),
    onSuccess: refresh,
  });
  return { create, rotate, remove };
}
