import { useNavigate } from "react-router";
import { Button, EmptyState, Notice, Spinner, formatDate } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useConversations, useDeleteConversation } from "../hooks/useConversations";

type Props = { tenant: string; app: string; current: string | null; onOpen: (id: string) => void; onDeleted: (id: string) => void };

/**
 * The person's conversations on every site. One of this site opens here;
 * one of another site opens that site with the conversation.
 */
export function HistoryList({ tenant, app, current, onOpen, onDeleted }: Props) {
  const list = useConversations(true);
  const remove = useDeleteConversation();
  const navigate = useNavigate();
  if (list.isPending) return <Spinner label="Loading your conversations…" />;
  if (list.isError) return <Notice tone="error">{describeError(list.error)}</Notice>;
  if (!list.data.length) {
    return <EmptyState title="No conversations yet">Your questions to the assistant are kept here, for you only.</EmptyState>;
  }
  const go = (id: string, other: string) => {
    if (other === app) onOpen(id);
    else navigate(`/t/${tenant}/sites/${other}/discovery?assistant=${encodeURIComponent(id)}`);
  };
  return (
    <div className="flex flex-col gap-2">
      {remove.isError ? <Notice tone="error">{describeError(remove.error)}</Notice> : null}
      <ul className="flex flex-col gap-2">
        {list.data.map((c) => (
          <li key={c.id} className={`flex items-start gap-2 rounded-[var(--radius-control)] border p-2 ${c.id === current ? "border-signal" : "border-rule"}`}>
            <button type="button" onClick={() => go(c.id, c.app)} className="min-w-0 flex-1 text-left">
              <span className="block truncate text-sm font-medium text-ink">{c.title}</span>
              <span className="block text-xs text-ink-soft">{c.app_name} · {formatDate(c.updated_at)}</span>
            </button>
            <Button variant="quiet" busy={remove.isPending && remove.variables === c.id} onClick={() => remove.mutate(c.id, { onSuccess: () => onDeleted(c.id) })}
              aria-label={`Delete the conversation "${c.title}"`}>
              Delete
            </Button>
          </li>
        ))}
      </ul>
    </div>
  );
}
