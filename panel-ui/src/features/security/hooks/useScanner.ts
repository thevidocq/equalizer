import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

export type ScannerInput = { endpoint: string; api_key?: string };

/** The tenant's vulnerability scanner (tenant administrators only). */
export function useScanner(tenant: string) {
  return useQuery({
    queryKey: ["scanner", tenant],
    queryFn: () => unwrap(api.GET("/tenants/{tenant}/scanner", { params: { path: { tenant } } })),
  });
}

export function useScannerActions(tenant: string) {
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: ["scanner", tenant] });
  const save = useMutation({
    mutationFn: (body: ScannerInput) => unwrap(api.PUT("/tenants/{tenant}/scanner", { params: { path: { tenant } }, body })),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: () => unwrap(api.DELETE("/tenants/{tenant}/scanner", { params: { path: { tenant } } })),
    onSuccess: refresh,
  });
  return { save, remove };
}
