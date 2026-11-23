import { Fragment, type ReactNode } from "react";
import type { App } from "@/shared/api/client";
import { formatDate } from "@/shared/ui";
import { eventKey, kindOf, subtitle, type LogEvent } from "./kinds";

type Props = {
  events: LogEvent[];
  apps: App[];
  selected: string | null;
  onSelect: (key: string) => void;
  detail: (e: LogEvent) => ReactNode; // shown under the open row
};

const mono = "font-mono text-[13px]";

export function eventTime(ts: string) {
  return formatDate(ts);
}

/**
 * The events, newest first. A row opens its detail right under it; the
 * table takes the page's width and never scrolls sideways: the request is
 * cut to fit, and the site and the address give way on narrow screens.
 */
export function EventTable({ events, apps, selected, onSelect, detail }: Props) {
  const name = (id?: string | null) => apps.find((a) => a.id === id)?.name ?? id ?? "";
  return (
    <table className="w-full table-fixed border-collapse text-sm">
      <caption className="sr-only">Security events, newest first</caption>
      <thead>
        <tr className="border-b border-rule-soft text-left text-[13px] text-ink-soft">
          <th scope="col" className="w-36 px-4 py-2.5 font-medium sm:w-52">Time</th>
          <th scope="col" className="w-40 px-2 py-2.5 font-medium sm:w-48">Event</th>
          <th scope="col" className="hidden w-44 px-2 py-2.5 font-medium lg:table-cell">Site</th>
          <th scope="col" className="px-2 py-2.5 font-medium">Request</th>
          <th scope="col" className="hidden w-36 px-2 py-2.5 font-medium md:table-cell">From</th>
          <th scope="col" className="w-16 px-4 py-2.5 text-right font-medium">Status</th>
        </tr>
      </thead>
      <tbody>
        {events.map((e, i) => {
          const k = kindOf(e.type);
          const key = eventKey(e);
          const on = key === selected;
          return (
            <Fragment key={`${key}|${i}`}>
              <tr onClick={() => onSelect(key)}
                className={`cursor-pointer border-b border-rule-soft ${on ? "bg-signal-soft" : "hover:bg-paper"}`}>
                <td className="px-4 py-2 tabular-nums text-ink-soft">
                  <time dateTime={e.ts}>{eventTime(e.ts)}</time>
                </td>
                <td className="overflow-hidden px-2 py-2">
                  <button type="button" onClick={(ev) => { ev.stopPropagation(); onSelect(key); }}
                    aria-expanded={on} className="flex w-full min-w-0 flex-col text-left">
                    <span className="flex items-center gap-1.5 font-medium">
                      <span aria-hidden="true" className="size-2 shrink-0 rounded-full" style={{ background: k.color }} />
                      {k.label}
                    </span>
                    {subtitle(e) ? <span className="truncate text-[13px] text-ink-soft">{subtitle(e)}</span> : null}
                  </button>
                </td>
                <td className="hidden truncate px-2 py-2 lg:table-cell">{e.app ? name(e.app) : e.connector ? `Connector ${e.connector}` : ""}</td>
                <td className={`truncate px-2 py-2 ${mono}`} title={e.path ?? ""}>{e.method} {e.path}</td>
                <td className={`hidden truncate px-2 py-2 md:table-cell ${mono}`}>{e.src_ip}</td>
                <td className="px-4 py-2 text-right tabular-nums">{e.status}</td>
              </tr>
              {on ? (
                <tr className="border-b border-rule-soft bg-paper">
                  <td colSpan={6} className="px-4 py-4">{detail(e)}</td>
                </tr>
              ) : null}
            </Fragment>
          );
        })}
      </tbody>
    </table>
  );
}
