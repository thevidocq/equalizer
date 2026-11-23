import { useEffect, useRef, useState } from "react";
import { Button, Notice, Spinner } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useAssistant } from "../hooks/useAssistant";
import { useConversations } from "../hooks/useConversations";
import { Composer } from "./Composer";
import { Conversation } from "./Conversation";
import { HistoryList } from "./HistoryList";
import { Suggestions } from "./Suggestions";

type Props = {
  tenant: string;
  app: string;
  siteName: string;
  mapVersion: number | null;
  canDraft: boolean;
  open: boolean;
  initialConversation: string | null; // from History, on another site
  onClose: () => void;
};

/**
 * The assistant of one site, on the right of the page on wide screens and
 * over the whole page on phones. It opens on the person's latest
 * conversation about this site; History has all of theirs.
 */
export function AssistantPanel({ tenant, app, siteName, mapVersion, canDraft, open, initialConversation, onClose }: Props) {
  const chat = useAssistant(app);
  const [view, setView] = useState<"chat" | "history">("chat");
  const list = useConversations(open);
  const resumed = useRef(false);
  useEffect(() => {
    if (!open || resumed.current) return;
    if (initialConversation) { resumed.current = true; chat.open(initialConversation); return; }
    if (!list.data) return;
    resumed.current = true;
    const latest = list.data.find((c) => c.app === app);
    if (latest) chat.open(latest.id);
  }, [open, initialConversation, list.data, app, chat]);
  useEffect(() => {
    // the page makes room for the panel on wide screens (see Shell)
    document.documentElement.style.setProperty("--assistant-width", open ? "26rem" : "0px");
    if (!open) return;
    const esc = (e: KeyboardEvent) => {
      // a dialog open over the page (Delete site…) takes its own Escape
      if (e.key === "Escape" && !document.querySelector('[role="dialog"], [role="alertdialog"]')) onClose();
    };
    window.addEventListener("keydown", esc);
    return () => {
      window.removeEventListener("keydown", esc);
      document.documentElement.style.setProperty("--assistant-width", "0px");
    };
  }, [open, onClose]);
  if (!open) return null;
  const openOne = (id: string) => { chat.open(id); setView("chat"); };
  return (
    <aside id="assistant-panel" aria-label={`Assistant for ${siteName}`}
      className="fixed inset-0 z-40 flex flex-col bg-surface lg:inset-y-0 lg:left-auto lg:right-0 lg:z-20 lg:w-[26rem] lg:border-l lg:border-rule">
      <header className="flex items-start justify-between gap-3 border-b border-rule-soft px-4 py-3">
        <div>
          <h2 className="font-semibold text-ink">Assistant · {siteName}</h2>
          <p className="mt-1 flex flex-wrap gap-1.5 text-xs text-ink-soft">
            <span className="rounded-full bg-paper px-2 py-0.5">{mapVersion ? `Map v${mapVersion}` : "No published map"}</span>
            <span className="rounded-full bg-paper px-2 py-0.5">Report · logs · history</span>
          </p>
        </div>
        <div className="flex flex-wrap items-center justify-end gap-x-3 gap-y-1">
          <Button variant="quiet" aria-pressed={view === "history"} onClick={() => setView(view === "history" ? "chat" : "history")}>
            {view === "history" ? "Back" : "History"}
          </Button>
          {view === "chat" && chat.messages.length ? <Button variant="quiet" onClick={chat.startNew}>New</Button> : null}
          <Button variant="secondary" onClick={onClose} aria-label="Close the assistant">Close</Button>
        </div>
      </header>
      <div className="flex-1 overflow-y-auto px-4 py-4">
        {view === "history" ? (
          <HistoryList tenant={tenant} app={app} current={chat.conversation} onOpen={openOne}
            onDeleted={(id) => { if (id === chat.conversation) chat.startNew(); }} />
        ) : chat.loading ? <Spinner label="Opening the conversation…" />
          : chat.loadError && !chat.messages.length
            ? <Notice tone="error">This conversation is no longer kept ({describeError(chat.loadError)}). Ask a new question below.</Notice>
            : chat.messages.length
              ? <Conversation tenant={tenant} app={app} messages={chat.messages} busy={chat.busy} canDraft={canDraft} />
              : <Suggestions onPick={chat.send} />}
      </div>
      {view === "chat" ? (
        <footer className="border-t border-rule-soft px-4 py-3">
          <Composer busy={chat.busy} onSend={chat.send} />
          <p className="mt-2 text-xs text-ink-soft">
            Only you see your conversations. Visitor addresses reach the AI as codes (V-…), never as IPs. The AI can be
            wrong: check before you publish.
          </p>
        </footer>
      ) : null}
    </aside>
  );
}
