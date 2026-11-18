import type { App } from "@/shared/api/client";
import { kindOf, reasonLabel, type LogEvent } from "./kinds";

type Props = { events: LogEvent[]; apps: App[]; selected: number; onSelect: (i: number) => void };

const mono = "font-mono text-[13px]";

export function eventTime(ts: string) {
  return new Date(ts).toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

/** The events, newest first; a row opens its detail. */
export function EventTable({ events, apps, selected, onSelect }: Props) {
  const name = (id?: string | null) => apps.find((a) => a.id === id)?.name ?? id ?? "";
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[640px] border-collapse text-sm">
        <caption className="sr-only">Security events, newest first</caption>
        <thead>
          <tr className="border-b border-rule-soft text-left text-[13px] text-ink-soft">
            <th scope="col" className="px-4 py-2.5 font-medium">Time</th>
            <th scope="col" className="px-2 py-2.5 font-medium">Event</th>
            <th scope="col" className="px-2 py-2.5 font-medium">Site</th>
            <th scope="col" className="px-2 py-2.5 font-medium">Request</th>
            <th scope="col" className="px-2 py-2.5 font-medium">From</th>
            <th scope="col" className="px-4 py-2.5 text-right font-medium">Status</th>
          </tr>
        </thead>
        <tbody>
          {events.map((e, i) => {
            const k = kindOf(e.type);
            return (
              <tr key={`${e.ts}-${e.request_id ?? i}`} onClick={() => onSelect(i)}
                className={`cursor-pointer border-b border-rule-soft ${i === selected ? "bg-signal-soft" : "hover:bg-paper"}`}>
                <td className="whitespace-nowrap px-4 py-2 tabular-nums text-ink-soft">
                  <time dateTime={e.ts}>{eventTime(e.ts)}</time>
                </td>
                <td className="px-2 py-2">
                  <button type="button" onClick={(ev) => { ev.stopPropagation(); onSelect(i); }}
                    aria-pressed={i === selected} className="flex flex-col text-left">
                    <span className="flex items-center gap-1.5 font-medium">
                      <span aria-hidden="true" className="size-2 rounded-full" style={{ background: k.color }} />
                      {k.label}
                    </span>
                    <span className="text-[13px] text-ink-soft">{reasonLabel(e)}</span>
                  </button>
                </td>
                <td className="whitespace-nowrap px-2 py-2">{name(e.app)}</td>
                <td className={`max-w-60 truncate px-2 py-2 ${mono}`} title={e.path ?? ""}>{e.method} {e.path}</td>
                <td className={`whitespace-nowrap px-2 py-2 ${mono}`}>{e.src_ip}</td>
                <td className="px-4 py-2 text-right tabular-nums">{e.status}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
