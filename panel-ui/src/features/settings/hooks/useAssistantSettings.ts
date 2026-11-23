import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

/** How long the assistant's conversations are kept. */
export function useAssistantSettings(tenant: string) {
  return useQuery({
    queryKey: ["assistant-settings", tenant],
    queryFn: () => unwrap(api.GET("/tenants/{tenant}/assistant", { params: { path: { tenant } } })),
  });
}

export function useSaveAssistantSettings(tenant: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (keep_days: number) => unwrap(api.PUT("/tenants/{tenant}/assistant", { params: { path: { tenant } }, body: { keep_days } })),
    onSuccess: (data) => {
      qc.setQueryData(["assistant-settings", tenant], data);
      void qc.invalidateQueries({ queryKey: ["assistant-conversations"] });
    },
  });
}
