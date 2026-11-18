import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

export type TenantInput = { id: string; name: string; admin_email: string; admin_name: string };

/** The tenants the caller may see: all for the platform, its own otherwise. */
export function useTenants() {
  return useQuery({ queryKey: ["tenants"], queryFn: async () => (await unwrap(api.GET("/tenants"))).items });
}

/** The platform's high-level view: names, site count and status only. */
export function useTenantOverview() {
  return useQuery({ queryKey: ["tenants", "overview"], queryFn: async () => (await unwrap(api.GET("/platform/tenants"))).items });
}

export function useCreateTenant() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: TenantInput) => unwrap(api.POST("/tenants", { body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tenants"] }),
  });
}

/** New invitation for a tenant's first administrator, before anyone joined. */
export function useReinvite(tenant: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { admin_email: string; admin_name: string }) =>
      unwrap(api.POST("/platform/tenants/{tenant}/invitation", { params: { path: { tenant } }, body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tenants"] }),
  });
}
