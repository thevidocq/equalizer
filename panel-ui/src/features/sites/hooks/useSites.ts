import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

export function useApps(tenant: string) {
  return useQuery({
    queryKey: ["apps", tenant],
    queryFn: async () => (await unwrap(api.GET("/tenants/{tenant}/apps", { params: { path: { tenant } } }))).items,
    enabled: !!tenant,
  });
}

export function useApp(app: string) {
  return useQuery({
    queryKey: ["app", app],
    queryFn: () => unwrap(api.GET("/apps/{app}", { params: { path: { app } } })),
    enabled: !!app,
  });
}

export function useCreateApp(tenant: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { id: string; name: string; public_domain: string }) =>
      unwrap(api.POST("/tenants/{tenant}/apps", { params: { path: { tenant } }, body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["apps", tenant] }),
  });
}
