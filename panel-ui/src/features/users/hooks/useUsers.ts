import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

export type Role = "admin" | "editor" | "approver" | "auditor";

export function useUsers(tenant: string) {
  return useQuery({
    queryKey: ["users", tenant],
    queryFn: async () => (await unwrap(api.GET("/tenants/{tenant}/users", { params: { path: { tenant } } }))).items,
  });
}

export function useCreateUser(tenant: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { email: string; name: string; password: string; role: Role }) =>
      unwrap(api.POST("/tenants/{tenant}/users", { params: { path: { tenant } }, body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["users", tenant] }),
  });
}
