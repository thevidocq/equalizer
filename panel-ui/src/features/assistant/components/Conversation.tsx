import { useEffect, useRef } from "react";
import { Spinner } from "@/shared/ui";
import type { Message } from "../hooks/useAssistant";
import { Markdown } from "../markdown/Markdown";
import { Card } from "../cards/Card";
import { ProposalCard } from "./ProposalCard";

type Props = { tenant: string; app: string; messages: Message[]; busy: boolean; canDraft: boolean };

/** The questions and answers, the latest kept in view. */
export function Conversation({ tenant, app, messages, busy, canDraft }: Props) {
  const end = useRef<HTMLDivElement>(null);
  // braces: a browser's scrollIntoView may return a Promise, which React would
  // then call as the effect's cleanup ("is not a function")
  useEffect(() => { void end.current?.scrollIntoView?.({ block: "end" }); }, [messages.length, busy]);
  return (
    <div className="flex flex-col gap-4" aria-live="polite">
      {messages.map((m, i) => m.role === "user" ? (
        <div key={i} className="ml-8 self-end rounded-[var(--radius-control)] bg-ink px-3 py-2 text-sm text-paper whitespace-pre-wrap break-words">
          {m.text}
        </div>
      ) : (
        <div key={i} className="flex flex-col gap-3 text-sm text-ink">
          {m.failed ? <p role="alert" className="text-state-rejected">{m.text}</p> : <Markdown text={m.text} />}
          {m.cards?.map((c, j) => <Card key={`c${j}`} tenant={tenant} app={app} card={c} />)}
          {m.proposals?.map((p) => <ProposalCard key={p.id} tenant={tenant} app={app} proposal={p} canDraft={canDraft} />)}
        </div>
      ))}
      {busy ? (
        <p className="text-sm text-ink-soft"><Spinner label="Reading the map, the report and the logs…" /></p>
      ) : null}
      <div ref={end} />
    </div>
  );
}
