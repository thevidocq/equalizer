import { useState, type FormEvent } from "react";
import { Button, Notice, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useOIDCSecret } from "../hooks/useSiteLogin";

/** The client secret: saved encrypted, never shown again, not in the map. */
export function SecretForm({ app, has, canEdit }: { app: string; has: boolean; canEdit: boolean }) {
  const { save, remove } = useOIDCSecret(app);
  const toast = useToast();
  const [value, setValue] = useState("");
  function submit(e: FormEvent) {
    e.preventDefault();
    save.mutate(value, { onSuccess: () => { setValue(""); toast("Client secret saved"); } });
  }
  const error = save.error ?? remove.error;
  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      {error ? <Notice tone="error">{describeError(error)}</Notice> : null}
      <TextField label="Client secret" type="password" autoComplete="off" value={value} disabled={!canEdit}
        onChange={(e) => setValue(e.target.value)} placeholder={has ? "Saved: type a new one to replace it" : "Not set"}
        hint="Encrypted, never shown again, given only to the gateways. Leave it unset for a public client (PKCE only)." />
      {canEdit ? (
        <div className="flex gap-2">
          <Button type="submit" busy={save.isPending} disabled={!value.trim()}>Save secret</Button>
          {has ? <Button variant="danger" busy={remove.isPending} onClick={() => remove.mutate(undefined, { onSuccess: () => toast("Client secret deleted") })}>Delete secret</Button> : null}
        </div>
      ) : null}
    </form>
  );
}
