import { Notice } from "@/shared/ui";
import type { Connector } from "@/shared/api/client";

/** What to do when a connector runs an older release than the platform. */
export function UpdateNotice({ list, platform }: { list: Connector[]; platform: string }) {
  const old = list.filter((c) => c.outdated).map((c) => c.id);
  if (!old.length) return null;
  return (
    <Notice tone="info">
      {old.join(", ")} {old.length === 1 ? "runs" : "run"} an older release than the platform ({platform}): some features do
      not work through {old.length === 1 ? "it" : "them"} (e.g. the site&apos;s own HTTP login). On its server:{" "}
      <code className="font-mono">docker pull ghcr.io/thevidocq/equalizer-connector:{platform}</code> and start it again with the
      same settings, or install the binary of that release.
    </Notice>
  );
}
