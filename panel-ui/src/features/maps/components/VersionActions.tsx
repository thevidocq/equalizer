import { Button, Notice, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import type { MapVersion, User } from "@/shared/api/client";
import { hasRole } from "@/features/auth";
import { useSite } from "@/features/exposure";
import { useMapAction, type MapAction } from "../hooks/useMaps";

const done: Record<MapAction, string> = {
  submit: "Sent for review", approve: "Approved", reject: "Rejected", publish: "Published", rollback: "Published again",
};

/** The next step of a version, offered only to who may take it. */
export function VersionActions({ v, tenant, me }: { v: MapVersion; tenant: string; me: User }) {
  const action = useMapAction(v.app, v.version);
  const tls = useSite(v.app).data?.tls;
  const noCert = tls?.public_scheme === "https" && tls.status === "missing";
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
        {noCert ? (
          <Notice tone="error">
            The site has no certificate yet: once published, visitors get no HTTPS until it is uploaded in Site → Certificate.
          </Notice>
        ) : null}
        <div><Button variant="primary" busy={action.isPending} onClick={() => run("publish")}>Publish</Button></div>
        <p className="text-sm text-ink-soft">The gateways pick it up within 15 seconds; the published version is retired.</p>
      </div>
    );
  } else if (v.status === "retired" && v.published_at && decider) {
    body = (
      <div className="flex flex-col gap-2">
        <div><Button busy={action.isPending} onClick={() => {
          if (window.confirm(`Publish version ${v.version} again? The current version is retired.`)) run("rollback");
        }}>Roll back to this version</Button></div>
        <p className="text-sm text-ink-soft">For when a new version breaks the site: this one was approved before, it goes live at once.</p>
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
