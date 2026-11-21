import { useState, type FormEvent } from "react";
import type { App } from "@/shared/api/client";
import { Button, Notice, SelectField, formatDate, timeZone, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useSchedule, type Frequency } from "../hooks/useDiscovery";

const days = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
const hours = Array.from({ length: 24 }, (_, h) => h);

/** Continuous discovery: how often, at what time, and when it runs next. */
export function ScheduleForm({ site }: { site: App }) {
  const at = site.discovery_time;
  const [freq, setFreq] = useState<Frequency>(site.discovery_schedule ?? "off");
  const [hour, setHour] = useState(at?.hour ?? 3);
  const [weekday, setWeekday] = useState(at?.weekday ?? 1);
  const save = useSchedule(site.id);
  const toast = useToast();
  const unpublished = site.published_version == null;
  const zone = timeZone();
  const changed = freq !== (site.discovery_schedule ?? "off") || (freq !== "off" &&
    (hour !== at?.hour || (freq === "weekly" && weekday !== at?.weekday) || (at?.zone ?? "UTC") !== zone));
  function submit(e: FormEvent) {
    e.preventDefault();
    save.mutate({ discovery_schedule: freq, discovery_hour: hour, discovery_weekday: weekday },
      { onSuccess: () => toast(freq === "off" ? "Continuous discovery turned off" : "Continuous discovery saved") });
  }
  return (
    <form onSubmit={submit} className="flex max-w-2xl flex-col gap-3">
      <div className="flex flex-wrap items-end gap-3">
        <SelectField label="Continuous discovery" value={freq} disabled={unpublished} onChange={(e) => setFreq(e.target.value as Frequency)}>
          <option value="off">Off</option><option value="daily">Every day</option><option value="weekly">Every week</option>
        </SelectField>
        {freq === "weekly" ? (
          <SelectField label="On" value={weekday} onChange={(e) => setWeekday(Number(e.target.value))}>
            {days.map((d, i) => <option key={d} value={i}>{d}</option>)}
          </SelectField>
        ) : null}
        {freq !== "off" ? (
          <SelectField label={`At (${zone})`} value={hour} onChange={(e) => setHour(Number(e.target.value))}>
            {hours.map((h) => <option key={h} value={h}>{String(h).padStart(2, "0")}:00</option>)}
          </SelectField>
        ) : null}
        <Button type="submit" variant="primary" busy={save.isPending} disabled={!changed || unpublished}>Save</Button>
      </div>
      <p className="text-sm text-ink-soft">
        {unpublished ? "Available once a map is published. " : "Explores again and proposes a draft only when something changed. "}
        {at?.next && site.discovery_schedule !== "off" ? `Next run: ${formatDate(at.next)}. ` : ""}
        {at?.last ? `Latest discovery: ${formatDate(at.last)}.` : ""}
      </p>
      {save.isError ? <Notice tone="error">{describeError(save.error)}</Notice> : null}
    </form>
  );
}
