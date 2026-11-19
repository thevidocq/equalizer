import type { Connector } from "@/shared/api/client";

/** Online or offline, with the release of each running connector. */
export function ConnectorState({ c }: { c: Connector }) {
  if (!c.online) return <span className="text-ink-soft">Offline</span>;
  const versions = [...new Set(c.versions ?? [])].join(", ") || "unknown";
  return (
    <span className="flex flex-col">
      <span><span className="mr-1.5 inline-block size-2 rounded-full bg-state-published align-middle" />Online, release {versions}</span>
      {c.outdated ? <span className="text-sm font-semibold text-state-review">Older than the platform: update it</span> : null}
    </span>
  );
}
