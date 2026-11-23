import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, unwrap } from "@/shared/api/client";

/** The person's own conversations, every site, newest first. */
export function useConversations(enabled: boolean) {
  return useQuery({
    queryKey: ["assistant-conversations"],
    queryFn: () => unwrap(api.GET("/assistant/conversations")).then((r) => r.items),
    enabled,
  });
}

/** Deletes one of them now. */
export function useDeleteConversation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => unwrap(api.DELETE("/assistant/conversations/{conversation}", { params: { path: { conversation: id } } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["assistant-conversations"] }),
  });
}
