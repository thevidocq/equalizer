import type { Schemas } from "@/shared/api/client";

/** One message of the conversation, as the panel shows it. */
export type Message = {
  role: "user" | "assistant";
  text: string;
  proposals?: Schemas["AssistantProposal"][];
  cards?: Schemas["AssistantCard"][];
  failed?: boolean; // the panel's own words, kept on this screen only
};
