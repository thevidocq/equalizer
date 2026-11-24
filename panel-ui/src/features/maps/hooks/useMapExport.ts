import { useMutation } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { save } from "../components/mapFile";

export type ExportFormat = "openapi" | "modsecurity";

const NAME: Record<ExportFormat, string> = { openapi: "openapi.json", modsecurity: "modsecurity.conf" };

/** Downloads a version written for another tool (OpenAPI 3, ModSecurity). */
export function useMapExport(app: string, version: number) {
  return useMutation({
    mutationFn: async (format: ExportFormat) => {
      const res = await api.GET("/apps/{app}/maps/{version}/export", {
        params: { path: { app, version }, query: { format } },
        parseAs: "blob",
      });
      if (!res.response.ok || !res.data) throw new Error("The export failed. Try again in a moment.");
      save(res.data as Blob, `${app}-map-v${version}.${NAME[format]}`);
    },
  });
}
