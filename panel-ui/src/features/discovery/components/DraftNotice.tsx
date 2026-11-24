import { Link } from "react-router";
import { Notice, Spinner } from "@/shared/ui";
import { VerificationPanel, progressText, useMapVersion, useVerification } from "@/features/maps";

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
  const version = useMapVersion(app, shown);
  if (checking && !v.isError) {
    return (
      <Notice tone="info">
        <Spinner label={`The agent is checking its draft on the gateway and correcting it${v.data?.progress ? `: ${progressText(v.data.progress)}` : ""}. The draft to review will appear here.`} />
      </Notice>
    );
  }
  const verified = v.data?.status === "done";
  // once a person acted on it, the draft is no longer one to review
  const status = version.data?.status;
  if (v.isError) {
    // a draft a person made from the report: the agent does not verify it
    return (
      <Notice tone="info">
        Draft to review: <Link className="font-semibold text-signal hover:underline" to={`../maps/${draft}`}>version {draft}</Link>.
        Review it and publish it.
      </Notice>
    );
  }
  const link = <Link className="font-semibold text-signal hover:underline" to={`../maps/${shown}`}>version {shown}</Link>;
  if (status === "published" || status === "retired" || status === "rejected") {
    const what = status === "published" ? "is published" : status === "rejected" ? "was rejected" : "was replaced by a newer version";
    return (
      <>
        <Notice tone="info">The draft of this discovery, {link}, {what}.</Notice>
        <VerificationPanel app={app} version={shown} canVerify={false} versionLink={(n) => `../maps/${n}`} />
      </>
    );
  }
  return (
    <>
      {verified ? (
        <Notice tone="info">Draft to review: {link}, verified by the agent. Review it and publish it.</Notice>
      ) : (
        <Notice tone="error">
          Draft to review: {link}. The agent could not verify it on the gateway; check it with care, or verify it
          again from its page before publishing it.
        </Notice>
      )}
      <VerificationPanel app={app} version={shown} canVerify={false} versionLink={(n) => `../maps/${n}`} />
    </>
  );
}
