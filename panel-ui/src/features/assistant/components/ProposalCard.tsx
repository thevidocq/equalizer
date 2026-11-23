import { Link } from "react-router";
import { Button } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useDraftProposal, type Proposal } from "../hooks/useAssistant";

type Props = { tenant: string; app: string; proposal: Proposal; canDraft: boolean };

/** A change the assistant proposed: an administrator makes it a draft, which the agent checks. */
export function ProposalCard({ tenant, app, proposal, canDraft }: Props) {
  const draft = useDraftProposal(app);
  const made = draft.data?.version ?? proposal.draft_version; // made now, or before (a kept conversation)
  return (
    <div className="rounded-[var(--radius-control)] border border-signal/30 bg-signal-soft p-3 text-sm">
      <p className="text-xs font-semibold uppercase tracking-wide text-ink-soft">Proposed change</p>
      <p className="mt-1 font-mono text-[0.8125rem] text-ink break-words">{proposal.summary}</p>
      {proposal.reason ? <p className="mt-1 text-ink-soft">{proposal.reason}</p> : null}
      <div className="mt-3 flex flex-wrap items-center gap-3">
        {made ? (
          <Link className="font-semibold text-signal hover:underline" to={`/t/${tenant}/sites/${app}/maps/${made}`}>
            Draft v{made} created: open it
          </Link>
        ) : canDraft ? (
          <Button variant="primary" busy={draft.isPending} onClick={() => draft.mutate(proposal.id)}>
            Create a draft and check it
          </Button>
        ) : (
          <span className="text-ink-soft">An administrator can make it a draft.</span>
        )}
      </div>
      {draft.isError ? <p role="alert" className="mt-2 text-state-rejected">{describeError(draft.error)}</p> : null}
      <p className="mt-2 text-xs text-ink-soft">A draft only: nothing changes until an administrator publishes it.</p>
    </div>
  );
}
