import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap, type Schemas } from "@/shared/api/client";

export type SourceReader = Schemas["SourceReader"];
export type SourceRead = Schemas["SourceRead"];
export type SourceUpdate = Schemas["SourceReaderUpdate"];

/** A read waiting or going on. */
export function reading(s: SourceReader | undefined): boolean {
  return s?.last_read?.status === "queued" || s?.last_read?.status === "running";
}

/**
 * The site's source reader. Refreshed every 10 seconds while a read goes on,
 * every 30 while it is installed and not answering yet.
 */
export function useSource(app: string) {
  return useQuery({
    queryKey: ["source", app],
    queryFn: () => unwrap(api.GET("/apps/{app}/source", { params: { path: { app } } })),
    refetchInterval: (q) => {
      const s = q.state.data;
      if (reading(s)) return 10_000;
      return s && s.configured && s.status !== "ready" ? 30_000 : false;
    },
  });
}

export function useSourceActions(app: string) {
  const qc = useQueryClient();
  const path = { params: { path: { app } } };
  const refresh = () => qc.invalidateQueries({ queryKey: ["source", app] });
  const save = useMutation({
    mutationFn: (body: SourceUpdate) => unwrap(api.PUT("/apps/{app}/source", { ...path, body })),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: () => unwrap(api.DELETE("/apps/{app}/source", path)),
    onSuccess: refresh,
  });
  const token = useMutation({
    mutationFn: () => unwrap(api.POST("/apps/{app}/source/token", path)),
    onSuccess: refresh,
  });
  const read = useMutation({
    mutationFn: () => unwrap(api.POST("/apps/{app}/source/reads", path)),
    onSuccess: refresh,
  });
  return { save, remove, token, read };
}
