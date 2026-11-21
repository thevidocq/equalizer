import { useMemo, useState, type FormEvent } from "react";
import { Button, Notice, formatDate, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useSetTimeZone } from "@/features/auth";

function zones(): string[] {
  const all = typeof Intl.supportedValuesOf === "function" ? Intl.supportedValuesOf("timeZone") : [];
  return ["UTC", ...all.filter((z) => z !== "UTC")];
}

/** The time zone every date of the panel and of the alert emails is written in. */
export function TimeZoneForm({ current }: { current: string }) {
  const list = useMemo(zones, []);
  const [zone, setZone] = useState(current);
  const save = useSetTimeZone();
  const toast = useToast();
  const browser = Intl.DateTimeFormat().resolvedOptions().timeZone;
  function submit(e: FormEvent) {
    e.preventDefault();
    save.mutate(zone, { onSuccess: () => toast("Time zone saved") });
  }
  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      <h2 className="font-semibold">Time zone</h2>
      <p className="text-sm text-ink-soft">Dates in the panel and in your alert emails are written in this time zone. Now: {formatDate(new Date().toISOString())}.</p>
      {save.isError ? <Notice tone="error">{describeError(save.error)}</Notice> : null}
      <label className="flex flex-col gap-1 text-sm">
        Time zone
        <select value={zone} onChange={(e) => setZone(e.target.value)}
          className="min-h-10 rounded-[var(--radius-control)] border border-rule bg-surface px-2 text-ink">
          {list.includes(zone) ? null : <option value={zone}>{zone}</option>}
          {list.map((z) => <option key={z} value={z}>{z.replaceAll("_", " ")}</option>)}
        </select>
      </label>
      <div className="flex flex-wrap gap-2">
        <Button type="submit" variant="primary" busy={save.isPending} disabled={zone === current}>Save time zone</Button>
        {browser && browser !== zone ? (
          <Button type="button" variant="quiet" onClick={() => setZone(browser)}>Use this device's ({browser.replaceAll("_", " ")})</Button>
        ) : null}
      </div>
    </form>
  );
}
