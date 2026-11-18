import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

export function useMfaSetup() {
  return useMutation({ mutationFn: () => unwrap(api.POST("/auth/mfa/setup")) });
}

export function useMfaEnable() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (code: string) => unwrap(api.POST("/auth/mfa/enable", { body: { code } })),
    // The session is complete only after the user saw the recovery codes:
    // the "me" query is refreshed when they continue.
    onSuccess: () => qc.removeQueries({ queryKey: ["tenants"] }),
  });
}

export function useMfaVerify() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (code: string) => unwrap(api.POST("/auth/mfa/verify", { body: { code } })),
    onSuccess: (user) => qc.setQueryData(["me"], user),
  });
}
