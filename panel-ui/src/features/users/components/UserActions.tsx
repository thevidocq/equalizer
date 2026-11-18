import { useState } from "react";
import { Button, Dialog, Notice, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import type { User } from "@/shared/api/client";
import { useUserActions } from "../hooks/useUsers";

type Pending = { user: User; kind: "deactivate" | "reset" } | null;

const copy = {
  deactivate: {
    title: (u: User) => `Deactivate ${u.name}?`,
    body: "They are logged out and can no longer log in. Their past actions stay in the activity log.",
    button: "Deactivate",
    done: (u: User) => `${u.name} deactivated`,
  },
  reset: {
    title: (u: User) => `Reset the authenticator of ${u.name}?`,
    body: "Use it when they lost their phone. They are logged out and set up the app again at the next login.",
    button: "Reset authenticator",
    done: (u: User) => `Authenticator of ${u.name} reset`,
  },
};

export function UserActions({ tenant, user, isMe }: { tenant: string; user: User; isMe: boolean }) {
  const { deactivate, resetMfa } = useUserActions(tenant);
  const toast = useToast();
  const [pending, setPending] = useState<Pending>(null);
  const mutation = pending?.kind === "reset" ? resetMfa : deactivate;
  if (isMe) return <span className="text-sm text-ink-soft">You</span>;

  return (
    <span className="inline-flex gap-2">
      {user.mfa_enabled ? <Button onClick={() => setPending({ user, kind: "reset" })}>Reset authenticator</Button> : null}
      <Button variant="danger" onClick={() => setPending({ user, kind: "deactivate" })}>Deactivate</Button>
      <Dialog open={!!pending} onOpenChange={(o) => { if (!o) { setPending(null); mutation.reset(); } }}
        title={pending ? copy[pending.kind].title(pending.user) : ""} description={pending ? copy[pending.kind].body : undefined}>
        {mutation.isError ? <div className="mb-4"><Notice tone="error">{describeError(mutation.error)}</Notice></div> : null}
        <div className="flex justify-end gap-2">
          <Button onClick={() => setPending(null)}>Cancel</Button>
          <Button variant="danger" busy={mutation.isPending} onClick={() => pending && mutation.mutate(pending.user.id ?? 0, {
            onSuccess: () => { toast(copy[pending.kind].done(pending.user)); setPending(null); },
          })}>{pending ? copy[pending.kind].button : ""}</Button>
        </div>
      </Dialog>
    </span>
  );
}
