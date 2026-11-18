import type { Schemas } from "@/shared/api/client";

type Site = Schemas["TenantOverview"]["sites"][number];

const label = { protected: "Protected", setting_up: "Being set up" } as const;

/** Site names with a status dot: the only detail the platform sees. */
export function SiteDots({ sites }: { sites: Site[] }) {
  if (sites.length === 0) return <span className="text-ink-soft">None yet</span>;
  return (
    <ul className="flex flex-wrap gap-x-4 gap-y-1">
      {sites.map((s) => (
        <li key={s.name} className="inline-flex items-center gap-2" title={label[s.status]}>
          <span aria-hidden="true" className={`h-2 w-2 rounded-full ${s.status === "protected" ? "bg-state-published" : "bg-state-retired"}`} />
          {s.name}
          <span className="sr-only">({label[s.status]})</span>
        </li>
      ))}
    </ul>
  );
}
