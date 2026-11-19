import { useMutation, useQuery } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

/** Asks for a reset link; the answer is the same whether the account exists or not. */
export function useRequestReset() {
  return useMutation({
    mutationFn: (email: string) => unwrap(api.POST("/auth/password-reset", { body: { email } })),
  });
}

/** Whether a reset link still works. */
export function useResetLink(token: string) {
  return useQuery({
    queryKey: ["reset", token],
    queryFn: () => unwrap(api.GET("/auth/password-reset/{token}", { params: { path: { token } } })),
    retry: false,
  });
}

/** Sets the new password; every session of the account ends. */
export function useFinishReset(token: string) {
  return useMutation({
    mutationFn: (password: string) =>
      unwrap(api.POST("/auth/password-reset/{token}", { params: { path: { token } }, body: { password } })),
  });
}
