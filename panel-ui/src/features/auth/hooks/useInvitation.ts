import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

/** What an invitation link offers, read before signing up. */
export function useInvitation(token: string) {
  return useQuery({
    queryKey: ["invitation", token],
    queryFn: () => unwrap(api.GET("/invitations/{token}", { params: { path: { token } } })),
  });
}

/** Accepting creates the account and logs in. */
export function useAcceptInvitation(token: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (password: string) =>
      unwrap(api.POST("/invitations/{token}/accept", { params: { path: { token } }, body: { password } })),
    onSuccess: () => qc.clear(),
  });
}
