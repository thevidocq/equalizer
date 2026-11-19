import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

export type AlertInput = { enabled: boolean; extra_emails: string[] };

/** Email alerts when a site stops working (tenant administrators only). */
export function useAlerts(tenant: string) {
  return useQuery({
    queryKey: ["alerts", tenant],
    queryFn: () => unwrap(api.GET("/tenants/{tenant}/alerts", { params: { path: { tenant } } })),
  });
}

export function useSaveAlerts(tenant: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: AlertInput) => unwrap(api.PUT("/tenants/{tenant}/alerts", { params: { path: { tenant } }, body })),
    onSuccess: (data) => qc.setQueryData(["alerts", tenant], data),
  });
}
