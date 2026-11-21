import { useQuery } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

/** The platform's release (and, for platform administrators, each service's). */
export function useAbout() {
  return useQuery({ queryKey: ["about"], queryFn: () => unwrap(api.GET("/about")), staleTime: 5 * 60_000 });
}
