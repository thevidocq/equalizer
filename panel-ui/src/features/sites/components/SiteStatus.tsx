import type { App } from "@/shared/api/client";

/** Green once a map is published (the site is protected), grey before. */
export function SiteStatus({ app, withLabel = false }: { app: App; withLabel?: boolean }) {
  const live = app.published_version != null;
  const label = live ? `Protected, map version ${app.published_version}` : "No map published yet";
  return (
    <span className="inline-flex items-center gap-2" title={label}>
      <span aria-hidden="true" className={`h-2 w-2 rounded-full ${live ? "bg-state-published" : "bg-state-retired"}`} />
      {withLabel ? <span className="text-sm text-ink-soft">{label}</span> : <span className="sr-only">{label}</span>}
    </span>
  );
}
