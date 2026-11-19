import { useMemo, useState, type FormEvent } from "react";
import { Button, Notice, TextField } from "@/shared/ui";
import { ApiError } from "@/shared/api/client";
import { describeError } from "@/shared/api/errors";
import { parseMap } from "./yaml";

type Props = {
  initial: string;
  busy: boolean;
  error: unknown;
  onSave: (doc: Record<string, unknown>, note: string) => void;
};

/** YAML editor: checks the syntax while typing; the panel checks the format on save. */
export function MapEditor({ initial, busy, error, onSave }: Props) {
  const [text, setText] = useState(initial);
  const [note, setNote] = useState("");
  const parsed = useMemo(() => parseMap(text), [text]);
  const details = error instanceof ApiError ? error.details : [];

  function submit(e: FormEvent) {
    e.preventDefault();
    if (parsed.ok) onSave(parsed.value, note);
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      {error ? (
        <Notice tone="error">
          <p>{describeError(error)}</p>
          {details.length > 0 ? (
            <ul className="mt-2 list-disc pl-5 font-mono text-[12.5px]">
              {details.map((d) => <li key={d}>{d}</li>)}
            </ul>
          ) : null}
        </Notice>
      ) : null}
      <div className="flex flex-col gap-1.5">
        <label htmlFor="map-yaml" className="text-sm font-semibold">Map (YAML)</label>
        <textarea id="map-yaml" spellCheck={false} value={text} onChange={(e) => setText(e.target.value)}
          aria-invalid={!parsed.ok || undefined} aria-describedby="map-yaml-msg"
          className={`scroll-box min-h-[28rem] w-full rounded-[var(--radius-control)] border bg-surface p-3 font-mono text-[13px] leading-[1.6]
            ${parsed.ok ? "border-rule" : "border-state-rejected"}`} />
        <p id="map-yaml-msg" className={`text-sm ${parsed.ok ? "text-ink-soft" : "text-state-rejected"}`}>
          {parsed.ok ? "Syntax is valid. The format is checked when you save." : parsed.message}
        </p>
      </div>
      <TextField label="What changed" maxLength={2000} value={note} onChange={(e) => setNote(e.target.value)}
        hint="Shown to the approver next to the version." />
      <div>
        <Button type="submit" variant="primary" busy={busy} disabled={!parsed.ok}>Save draft</Button>
      </div>
    </form>
  );
}
