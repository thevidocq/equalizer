import { Button } from "@/shared/ui";

/** Opens the assistant; pressed while it is open. */
export function AskAIButton({ open, onToggle }: { open: boolean; onToggle: () => void }) {
  return (
    <Button variant={open ? "primary" : "secondary"} onClick={onToggle} aria-expanded={open} aria-controls="assistant-panel">
      <svg aria-hidden="true" viewBox="0 0 16 16" className="h-4 w-4" fill="currentColor">
        <path d="M8 1l1.6 4.4L14 7l-4.4 1.6L8 13l-1.6-4.4L2 7l4.4-1.6z" />
        <path d="M13 11l.6 1.4L15 13l-1.4.6L13 15l-.6-1.4L11 13l1.4-.6z" />
      </svg>
      Ask AI
    </Button>
  );
}
