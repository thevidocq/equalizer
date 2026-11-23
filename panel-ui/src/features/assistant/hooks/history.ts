import type { Schemas } from "@/shared/api/client";

/** One message of the conversation, as the panel shows it. */
export type Message = {
  role: "user" | "assistant";
  text: string;
  proposals?: Schemas["AssistantProposal"][];
  failed?: boolean; // the panel's own words: never sent back to the model
};

const keep = 19; // the API takes 20 messages: the last ones, the new question included
const chars = 4000; // and 4000 characters each: a long answer goes back cut

const clip = (text: string) => { const c = Array.from(text); return c.length > chars ? c.slice(0, chars - 1).join("") + "…" : text; };

/** The messages to send: answered turns only, starting with the person's. */
export function history(messages: Message[], question: string) {
  const turns = messages.filter((m) => !m.failed).map(({ role, text }) => ({ role, text: clip(text) }));
  let start = Math.max(0, turns.length - keep);
  if (turns[start]?.role === "assistant") start += 1;
  return [...turns.slice(start), { role: "user" as const, text: question }];
}

