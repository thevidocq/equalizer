import { useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api, unwrap, type Schemas } from "@/shared/api/client";
import { describeError } from "@/shared/api/errors";
import type { Message } from "./history";

export type Proposal = Schemas["AssistantProposal"];
export type { Message };

/**
 * The person's conversation about one site. The server keeps it (private to
 * the person) and gives the model its past turns: the panel sends only the
 * new question and the conversation's id. An answer or a load that arrives
 * after the person moved to another conversation is dropped.
 */
export function useAssistant(app: string) {
  const qc = useQueryClient();
  const [conversation, setConversation] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const view = useRef(0); // which conversation is on screen; changes on open and New
  const ask = useMutation({
    mutationFn: (v: { question: string; conversation: string | null; view: number }) =>
      unwrap(api.POST("/apps/{app}/assistant", {
        params: { path: { app } }, body: { text: v.question, ...(v.conversation ? { conversation: v.conversation } : {}) },
      })),
    onMutate: ({ question }) => setMessages((m) => [...m, { role: "user", text: question }]),
    onSuccess: (a, v) => {
      void qc.invalidateQueries({ queryKey: ["assistant-conversations"] });
      if (v.view !== view.current) return; // kept in History all the same
      setConversation(a.conversation);
      setMessages((m) => [...m, { role: "assistant", text: a.text, proposals: a.proposals, cards: a.cards }]);
    },
    onError: (err, v) => {
      if (v.view !== view.current) return;
      setMessages((m) => [
        ...m.slice(0, -1), { role: "user", text: v.question, failed: true }, { role: "assistant", text: describeError(err), failed: true },
      ]);
    },
  });
  const load = useMutation({
    mutationFn: (v: { id: string; view: number }) =>
      unwrap(api.GET("/assistant/conversations/{conversation}", { params: { path: { conversation: v.id } } })),
    onSuccess: (c, v) => {
      if (v.view !== view.current) return;
      setConversation(c.conversation.id);
      setMessages(c.messages.map(({ role, text, cards, proposals }) => ({ role, text, cards, proposals })));
    },
  });
  const startNew = () => {
    view.current += 1;
    load.reset();
    setConversation(null);
    setMessages([]);
  };
  return {
    conversation,
    messages,
    busy: ask.isPending || load.isPending,
    loading: load.isPending,
    loadError: load.error,
    send: (question: string) => ask.mutate({ question, conversation, view: view.current }),
    open: (id: string) => {
      startNew();
      load.mutate({ id, view: view.current });
    },
    startNew,
  };
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
