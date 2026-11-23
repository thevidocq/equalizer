import { useState, type FormEvent, type KeyboardEvent } from "react";
import { Button } from "@/shared/ui";

/** The question box: Enter sends, Shift+Enter starts a new line. */
export function Composer({ busy, onSend }: { busy: boolean; onSend: (text: string) => void }) {
  const [text, setText] = useState("");
  const send = (e?: FormEvent) => {
    e?.preventDefault();
    const q = text.trim();
    if (!q || busy) return;
    onSend(q);
    setText("");
  };
  const onKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) send(e);
  };
  return (
    <form onSubmit={send} className="flex items-end gap-2">
      <label className="sr-only" htmlFor="assistant-question">Ask the assistant</label>
      <textarea
        id="assistant-question" rows={2} maxLength={4000} value={text} placeholder="Ask the assistant"
        onChange={(e) => setText(e.target.value)} onKeyDown={onKey}
        className="min-h-[2.75rem] flex-1 resize-none rounded-[var(--radius-control)] border border-rule bg-surface px-3 py-2 text-sm
          focus:border-signal focus:outline-none"
      />
      <Button type="submit" variant="primary" busy={busy} disabled={!text.trim()}>Send</Button>
    </form>
  );
}
