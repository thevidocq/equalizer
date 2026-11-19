import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

/** Addresses to register in the identity provider, and whether a secret is saved. */
export function useOIDCInfo(app: string) {
  return useQuery({
    queryKey: ["oidc", app],
    queryFn: () => unwrap(api.GET("/apps/{app}/oidc", { params: { path: { app } } })),
  });
}

export function useOIDCSecret(app: string) {
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: ["oidc", app] });
  const save = useMutation({
    mutationFn: (client_secret: string) =>
      unwrap(api.PUT("/apps/{app}/oidc/secret", { params: { path: { app } }, body: { client_secret } })),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: () => unwrap(api.DELETE("/apps/{app}/oidc/secret", { params: { path: { app } } })),
    onSuccess: refresh,
  });
  return { save, remove };
}
