import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

export type LLMInput = { provider: "anthropic" | "openai" | "local"; model: string; endpoint: string | null; api_key?: string };

/** The tenant's own AI model (tenant administrators only). */
export function useLLM(tenant: string, enabled = true) {
  return useQuery({
    queryKey: ["llm", tenant],
    queryFn: () => unwrap(api.GET("/tenants/{tenant}/llm", { params: { path: { tenant } } })),
    enabled,
  });
}

export function useLLMActions(tenant: string) {
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: ["llm", tenant] });
  const save = useMutation({
    mutationFn: (body: LLMInput) => unwrap(api.PUT("/tenants/{tenant}/llm", { params: { path: { tenant } }, body })),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: () => unwrap(api.DELETE("/tenants/{tenant}/llm", { params: { path: { tenant } } })),
    onSuccess: refresh,
  });
  return { save, remove };
}
