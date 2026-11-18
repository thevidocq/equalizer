import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

export type Role = "admin" | "editor" | "approver" | "auditor";

export function useUsers(tenant: string) {
  return useQuery({
    queryKey: ["users", tenant],
    queryFn: async () => (await unwrap(api.GET("/tenants/{tenant}/users", { params: { path: { tenant } } }))).items,
  });
}

export function useUserActions(tenant: string) {
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: ["users", tenant] });
  const deactivate = useMutation({
    mutationFn: (id: number) => unwrap(api.DELETE("/tenants/{tenant}/users/{id}", { params: { path: { tenant, id: String(id) } } })),
    onSuccess: refresh,
  });
  const resetMfa = useMutation({
    mutationFn: (id: number) => unwrap(api.POST("/tenants/{tenant}/users/{id}/reset-mfa", { params: { path: { tenant, id: String(id) } } })),
    onSuccess: refresh,
  });
  return { deactivate, resetMfa };
}
