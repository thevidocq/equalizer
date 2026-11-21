import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap, type App } from "@/shared/api/client";

/** Old public IP and exposure check of a site. */
export function useExposureActions(app: string) {
  const qc = useQueryClient();
  const store = (a: App) => {
    qc.setQueryData(["app", app], a);
    void qc.invalidateQueries({ queryKey: ["apps"] });
  };
  const setIp = useMutation({
    mutationFn: (legacy_ip: string | null) => unwrap(api.PATCH("/apps/{app}", { params: { path: { app } }, body: { legacy_ip } })),
    onSuccess: store,
  });
  const setNeverPublic = useMutation({
    mutationFn: (never_public: boolean) => unwrap(api.PATCH("/apps/{app}", { params: { path: { app } }, body: { never_public } })),
    onSuccess: store,
  });
  const check = useMutation({
    mutationFn: () => unwrap(api.POST("/apps/{app}/exposure/check", { params: { path: { app } } })),
    onSuccess: store,
  });
  const setAliases = useMutation({
    mutationFn: (aliases: string[]) => unwrap(api.PUT("/apps/{app}/aliases", { params: { path: { app } }, body: { aliases } })),
    onSuccess: store,
  });
  const setReach = useMutation({
    mutationFn: (body: { mode: "connector" | "direct"; connector_id?: string; address: string }) =>
      unwrap(api.PUT("/apps/{app}/reach", { params: { path: { app } }, body })),
    onSuccess: store,
  });
  return { setIp, setNeverPublic, check, setAliases, setReach };
}

/** The site, shared with the site header (same query key). */
export function useSite(app: string) {
  return useQuery({ queryKey: ["app", app], queryFn: () => unwrap(api.GET("/apps/{app}", { params: { path: { app } } })) });
}
