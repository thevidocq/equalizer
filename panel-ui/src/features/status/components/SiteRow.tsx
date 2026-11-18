import type { Schemas } from "@/shared/api/client";
import { State } from "./State";

type Site = Schemas["SiteStatus"];

function dayLabel(date: string) {
  return new Date(`${date}T00:00:00Z`).toLocaleDateString("en-GB", { weekday: "short", day: "numeric", timeZone: "UTC" });
}

/** One site: its overall state, what it needs, and the last 7 days. */
export function SiteRow({ site }: { site: Site }) {
  return (
    <li className="grid gap-4 border-b border-rule-soft py-4 md:grid-cols-[14rem_1fr_auto]">
      <div className="flex flex-col gap-0.5">
        <span className="font-semibold">{site.name}</span>
        <span className="font-mono text-[13px] text-ink-soft">{site.public_domain}</span>
        <span className="mt-1 text-sm"><State state={site.state} strong /></span>
      </div>
      <dl className="grid grid-cols-[7rem_minmax(0,1fr)] gap-x-3 gap-y-1 text-sm">
        {site.checks.map((c) => (
          <div key={c.name} className="contents">
            <dt className="text-ink-soft">{c.name}</dt>
            <dd className="flex flex-wrap gap-x-2"><State state={c.state} />{c.detail ? <span className="text-ink-soft">{c.detail}</span> : null}</dd>
          </div>
        ))}
      </dl>
      <div>
        <p className="mb-1 text-xs text-ink-soft">Last 7 days</p>
        <ol className="flex gap-1" aria-label="Problems per day, last 7 days">
          {site.days.map((d) => {
            const bad = d.legacy_errors + d.connector_drops;
            const words = bad ? `${d.legacy_errors} legacy errors, ${d.connector_drops} connector drops` : "no problems";
            return (
              <li key={d.date} title={`${dayLabel(d.date)}: ${words}`} aria-label={`${dayLabel(d.date)}: ${words}`}
                className="h-8 w-4 rounded-[2px]"
                style={{ background: d.connector_drops ? "#b42318" : d.legacy_errors ? "#d9a441" : "#b7d7c2" }} />
            );
          })}
        </ol>
      </div>
    </li>
  );
}
