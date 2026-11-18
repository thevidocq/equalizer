import { Button, Notice, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import type { MapVersion, User } from "@/shared/api/client";
import { hasRole } from "@/features/auth";
import { useMapAction, type MapAction } from "../hooks/useMaps";

const done: Record<MapAction, string> = {
  submit: "Sent for review", approve: "Approved", reject: "Rejected", publish: "Published",
};

/** The next step of a version, offered only to who may take it. */
export function VersionActions({ v, tenant, me }: { v: MapVersion; tenant: string; me: User }) {
  const action = useMapAction(v.app, v.version);
  const toast = useToast();
  const run = (a: MapAction) => action.mutate(a, { onSuccess: () => toast(`Version ${v.version}: ${done[a].toLowerCase()}`) });
  const isAuthor = v.author_id === me.id;
  const decider = hasRole(me, tenant, "admin", "approver");

  let body = null;
  if (v.status === "draft" && hasRole(me, tenant, "admin", "editor")) {
    body = <Button variant="primary" busy={action.isPending} onClick={() => run("submit")}>Send for review</Button>;
  } else if (v.status === "in_review" && decider && !isAuthor) {
    body = (
      <>
        <Button variant="primary" busy={action.isPending} onClick={() => run("approve")}>Approve</Button>
        <Button variant="danger" busy={action.isPending} onClick={() => run("reject")}>Reject</Button>
      </>
    );
  } else if (v.status === "in_review" && isAuthor) {
    body = <p className="text-sm text-ink-soft">Waiting for someone other than you to approve or reject it.</p>;
  } else if (v.status === "approved" && decider) {
    body = (
      <div className="flex flex-col gap-2">
        <div><Button variant="primary" busy={action.isPending} onClick={() => run("publish")}>Publish</Button></div>
        <p className="text-sm text-ink-soft">The gateways pick it up within 15 seconds; the published version is retired.</p>
      </div>
    );
  }
  return (
    <div className="flex flex-col gap-3">
      {action.isError ? <Notice tone="error">{describeError(action.error)}</Notice> : null}
      {body ? <div className="flex flex-wrap items-center gap-2">{body}</div> : null}
    </div>
  );
}
