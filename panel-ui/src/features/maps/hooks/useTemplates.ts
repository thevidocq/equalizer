import { useMutation, useQuery } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

export function useTemplates() {
  return useQuery({ queryKey: ["map-templates"], queryFn: async () => (await unwrap(api.GET("/map-templates"))).items, staleTime: Infinity });
}


export type TemplateInput = { template: string; mode: "connector" | "direct"; connector_id?: string; address: string; scheme: "https" | "http"; max_upload_mb: number };

/** A template filled for the site; nothing is saved. */
export function useFillTemplate(app: string) {
  return useMutation({
    mutationFn: async (body: TemplateInput) =>
      (await unwrap(api.POST("/apps/{app}/map-template", { params: { path: { app } }, body }))).document,
  });
}
