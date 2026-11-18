import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, unwrap, type User } from "@/shared/api/client";

/** The logged-in user; null when there is no session. */
export function useMe() {
  return useQuery({
    queryKey: ["me"],
    queryFn: async (): Promise<User | null> => {
      try {
        return await unwrap(api.GET("/auth/me"));
      } catch (e) {
        if (e instanceof ApiError && e.status === 401) return null;
        throw e;
      }
    },
    staleTime: 60_000,
  });
}

export function useLogin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { email: string; password: string }) => unwrap(api.POST("/auth/login", { body })),
    onSuccess: (user) => {
      qc.clear();
      qc.setQueryData(["me"], user);
    },
  });
}

export function useChangePassword() {
  return useMutation({
    mutationFn: (body: { current_password: string; new_password: string }) => unwrap(api.POST("/auth/password", { body })),
  });
}

export function useLogout() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.POST("/auth/logout")),
    onSettled: () => {
      qc.clear();
      qc.setQueryData(["me"], null);
    },
  });
}

/** Role helpers, mirroring panel-api's checks (the API decides anyway).
 * Platform users hold no tenant role: they never act inside a tenant. */
export function canManage(u: User | null | undefined, tenant: string) {
  return hasRole(u, tenant, "admin");
}

export function hasRole(u: User | null | undefined, tenant: string, ...roles: string[]) {
  return !!u && u.tenant === tenant && !!u.tenant_role && roles.includes(u.tenant_role);
}
