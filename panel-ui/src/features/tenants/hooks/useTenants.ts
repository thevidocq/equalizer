import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

export function useTenants() {
  return useQuery({ queryKey: ["tenants"], queryFn: async () => (await unwrap(api.GET("/tenants"))).items });
}

export function useCreateTenant() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { id: string; name: string }) => unwrap(api.POST("/tenants", { body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tenants"] }),
  });
}
