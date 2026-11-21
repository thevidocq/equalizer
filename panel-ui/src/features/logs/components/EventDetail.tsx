import { Link } from "react-router";
import type { App } from "@/shared/api/client";
import { Button, formatDate, useToast } from "@/shared/ui";
import { explanation, isRequest, kindOf, subtitle, type LogEvent } from "./kinds";

type Props = { event: LogEvent; apps: App[]; tenant: string; canEditMap: boolean; onOnlyIP: (ip: string) => void };

const mono = "font-mono overflow-wrap-anywhere break-all";

/** One event in plain words, with what can be done about it. */
export function EventDetail({ event: e, apps, tenant, canEditMap, onOnlyIP }: Props) {
  const toast = useToast();
  const k = kindOf(e.type);
  const site = apps.find((a) => a.id === e.app);
  const request = isRequest(e);
  const rows: [string, string | null | undefined, boolean][] = [
    ["When", formatDate(e.ts), false],
    ["Site", site?.name ?? e.app, false],
    ["Connector", e.connector, true],
    ["Request", e.path ? `${e.method ?? ""} ${e.path}` : null, true],
    ["From", e.src_ip, true],
    ["Reason code", e.reason, true],
    ["Map entry", request ? e.route ?? "none" : null, false],
    ["Map version", e.map_version ? `v${e.map_version}` : null, false],
    ["Request ID", e.request_id, true],
  ];
  const sub = subtitle(e);
  const unmapped = e.reason === "route_not_mapped" && site && canEditMap;
  return (
    <aside aria-label="Event detail" className="flex w-full max-w-sm flex-[1_1_320px] flex-col gap-3.5 rounded-[var(--radius-panel)] border border-rule-soft bg-surface p-[18px]">
      <div className="flex items-center gap-2">
        <span aria-hidden="true" className="size-2 rounded-full" style={{ background: k.color }} />
        <h2 className="text-[17px] font-semibold">{sub ? `${k.label}: ${sub}` : k.label}</h2>
      </div>
      <p className="text-sm">{explanation(e)}</p>
      <dl className="grid grid-cols-[max-content_minmax(0,1fr)] gap-x-3.5 gap-y-1.5 text-[13px]">
        {rows.filter(([, v]) => v).map(([label, v, isMono]) => (
          <div key={label} className="contents">
            <dt className="text-ink-soft">{label}</dt>
            <dd className={isMono ? mono : ""}>{v}</dd>
          </div>
        ))}
      </dl>
      <div className="flex flex-wrap gap-2">
        {unmapped ? (
          <Link to={`/t/${tenant}/sites/${site.id}/maps/new`}
            className="inline-flex items-center rounded-[var(--radius-control)] bg-signal px-3.5 py-2 text-sm font-semibold text-white hover:bg-[#183f94]">
            Add to the map
          </Link>
        ) : null}
        {e.src_ip ? <Button onClick={() => onOnlyIP(e.src_ip ?? "")}>Show this IP only</Button> : null}
        {e.request_id ? (
          <Button onClick={() => void navigator.clipboard.writeText(e.request_id ?? "").then(() => toast("Request ID copied"))}>
            Copy request ID
          </Button>
        ) : null}
      </div>
      {e.field || e.type === "request.blocked" ? (
        <p className="border-t border-rule-soft pt-2.5 text-xs text-ink-soft">
          Field values are never stored: only the field name and the rule that refused it.
        </p>
      ) : null}
    </aside>
  );
}
