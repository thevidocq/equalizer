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
  const check = useMutation({
    mutationFn: () => unwrap(api.POST("/apps/{app}/exposure/check", { params: { path: { app } } })),
    onSuccess: store,
  });
  return { setIp, check };
}

/** The site, shared with the site header (same query key). */
export function useSite(app: string) {
  return useQuery({ queryKey: ["app", app], queryFn: () => unwrap(api.GET("/apps/{app}", { params: { path: { app } } })) });
}
