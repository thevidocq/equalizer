import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api, unwrap, type Schemas } from "@/shared/api/client";
import { describeError } from "@/shared/api/errors";
import { history, type Message } from "./history";

export type Proposal = Schemas["AssistantProposal"];
export type { Message };

/** A conversation with the assistant about one site; it lives as long as the site's page. */
export function useAssistant(app: string) {
  const [messages, setMessages] = useState<Message[]>([]);
  const ask = useMutation({
    mutationFn: (question: string) =>
      unwrap(api.POST("/apps/{app}/assistant", { params: { path: { app } }, body: { messages: history(messages, question) } })),
    onMutate: (question) => setMessages((m) => [...m, { role: "user", text: question }]),
    onSuccess: (a) => setMessages((m) => [...m, { role: "assistant", text: a.text, proposals: a.proposals, cards: a.cards }]),
    onError: (err, question) => setMessages((m) => {
      // the question stays on screen; it is sent again with the next one only if it was answered
      const rest = m.slice(0, -1);
      return [...rest, { role: "user", text: question, failed: true }, { role: "assistant", text: describeError(err), failed: true }];
    }),
  });
  return { messages, ask, clear: () => setMessages([]) };
}

/** Turns a proposal into a draft the agent checks. */
export function useDraftProposal(app: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (proposal: string) =>
      unwrap(api.POST("/apps/{app}/assistant/proposals/{proposal}/draft", { params: { path: { app, proposal } } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["maps", app] }),
  });
}
