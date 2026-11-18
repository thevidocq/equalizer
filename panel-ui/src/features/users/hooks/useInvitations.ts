import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";
import type { Role } from "./useUsers";

export function useInvitations(tenant: string) {
  return useQuery({
    queryKey: ["invitations", tenant],
    queryFn: async () => (await unwrap(api.GET("/tenants/{tenant}/invitations", { params: { path: { tenant } } }))).items,
  });
}

export function useInvite(tenant: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { email: string; name: string; role: Role }) =>
      unwrap(api.POST("/tenants/{tenant}/invitations", { params: { path: { tenant } }, body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["invitations", tenant] }),
  });
}

export function useRevokeInvitation(tenant: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) =>
      unwrap(api.DELETE("/tenants/{tenant}/invitations/{id}", { params: { path: { tenant, id: String(id) } } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["invitations", tenant] }),
  });
}
