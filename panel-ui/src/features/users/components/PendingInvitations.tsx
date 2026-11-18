import { Button, Notice, Table, formatDate, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import type { Schemas } from "@/shared/api/client";
import { useInvitations, useRevokeInvitation } from "../hooks/useInvitations";
import { roleText } from "./roles";

type Invitation = Schemas["Invitation"];

/** Invitations not accepted yet; hidden when there are none. */
export function PendingInvitations({ tenant }: { tenant: string }) {
  const list = useInvitations(tenant);
  const revoke = useRevokeInvitation(tenant);
  const toast = useToast();
  if (!list.data?.length && !list.isError) return null;

  return (
    <section className="flex flex-col gap-3">
      <h2 className="text-base font-semibold">Waiting to join</h2>
      {list.isError ? <Notice tone="error">{describeError(list.error)}</Notice> : null}
      {revoke.isError ? <Notice tone="error">{describeError(revoke.error)}</Notice> : null}
      {list.data?.length ? (
        <Table<Invitation> caption="Pending invitations" rows={list.data} rowKey={(i) => String(i.id)} columns={[
          { header: "Name", cell: (i) => i.name },
          { header: "Email", cell: (i) => i.email },
          { header: "Role", cell: (i) => roleText[i.role].name },
          { header: "Link expires", cell: (i) => (i.sent === false
            ? <span className="text-state-rejected">Email not sent</span> : formatDate(i.expires_at)) },
          { header: "", className: "text-right", cell: (i) => (
            <Button busy={revoke.isPending && revoke.variables === i.id}
              onClick={() => revoke.mutate(i.id, { onSuccess: () => toast(`Invitation for ${i.email} revoked`) })}>
              Revoke
            </Button>
          ) },
        ]} />
      ) : null}
    </section>
  );
}
