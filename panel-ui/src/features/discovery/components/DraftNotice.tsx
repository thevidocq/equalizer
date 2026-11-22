import { Link } from "react-router";
import { Notice, Spinner } from "@/shared/ui";
import { VerificationPanel, useVerification } from "@/features/maps";

/**
 * The one draft a discovery proposes: the agent verifies it on the gateway
 * and corrects it before a person looks at it, so the person reviews the
 * agent's last version, never the one it is still correcting.
 */
export function DraftNotice({ app, draft }: { app: string; draft: number }) {
  const v = useVerification(app, draft);
  const checking = v.isPending || (v.data && ["queued", "running"].includes(v.data.status));
  // the discovery may still name the first draft until it is read again
  const shown = v.data?.corrected_version ?? draft;
  if (checking && !v.isError) {
    return (
      <Notice tone="info">
        <Spinner label={`The agent is checking its draft on the gateway and correcting it. The draft to review will appear here.`} />
      </Notice>
    );
  }
  return (
    <>
      <Notice tone="info">
        Draft to review: <Link className="font-semibold text-signal hover:underline" to={`../maps/${shown}`}>version {shown}</Link>,
        verified by the agent. Review it and publish it.
      </Notice>
      <VerificationPanel app={app} version={shown} canVerify={false} versionLink={(n) => `../maps/${n}`} />
    </>
  );
}
