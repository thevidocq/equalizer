import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

export type AdoptItem = { kind: "file"; path: string } | { kind: "key"; route: string; key: string };

/** Adds one thing visitors asked for to a new draft: the change comes from the discovery's report. */
export function useAdopt(app: string, discovery: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: AdoptItem) =>
      unwrap(api.POST("/apps/{app}/discoveries/{id}/adopt", { params: { path: { app, id: String(discovery) } }, body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["maps", app] }),
  });
}
