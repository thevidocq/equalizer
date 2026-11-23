import { useState, type FormEvent } from "react";
import { Button, Notice, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useSaveAssistantSettings } from "../hooks/useAssistantSettings";

/** How long the assistant's conversations are kept (each person sees only their own). */
export function AssistantForm({ tenant, keepDays }: { tenant: string; keepDays: number }) {
  const save = useSaveAssistantSettings(tenant);
  const toast = useToast();
  const [days, setDays] = useState(String(keepDays));
  const n = Number(days);
  const valid = Number.isInteger(n) && n >= 1 && n <= 365;

  function submit(e: FormEvent) {
    e.preventDefault();
    if (valid) save.mutate(n, { onSuccess: () => toast("Assistant settings saved") });
  }

  return (
    <form onSubmit={submit} className="flex max-w-xl flex-col gap-4 border-b border-rule-soft pb-6">
      <div>
        <h2 className="text-base font-semibold">Assistant</h2>
        <p className="mt-1 text-sm text-ink-soft">
          Each person's conversations with a site's assistant are kept for them only: nobody else reads them, other
          administrators included. They are deleted this many days after their last message.
        </p>
      </div>
      {save.isError ? <Notice tone="error">{describeError(save.error)}</Notice> : null}
      <TextField label="Keep conversations for (days)" type="number" inputMode="numeric" min={1} max={365} value={days}
        onChange={(e) => setDays(e.target.value)} error={valid ? undefined : "From 1 to 365 days."}
        hint="Lowering it deletes the older conversations at once." />
      <div><Button type="submit" variant="primary" busy={save.isPending} disabled={!valid}>Save</Button></div>
    </form>
  );
}
