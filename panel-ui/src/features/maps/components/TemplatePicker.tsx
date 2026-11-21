import { useState, type FormEvent } from "react";
import { Button, Notice, SelectField, TextField } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { ReachFields, emptyReach, useChosenConnector, type Reach } from "@/features/connectors";
import { useFillTemplate, useTemplates } from "../hooks/useTemplates";

type Props = { tenant: string; app: string; kind: string | null | undefined; onFill: (doc: Record<string, unknown>) => void };

/** Fills the editor with a starting map for the site's CMS. */
export function TemplatePicker({ tenant, app, kind, onFill }: Props) {
  const templates = useTemplates();
  const fill = useFillTemplate(app);
  const [template, setTemplate] = useState(kind ?? "generic");
  const [reach, setReach] = useState<Reach>(emptyReach);
  const [maxUpload, setMaxUpload] = useState(50);
  const chosen = templates.data?.find((t) => t.id === template);
  const connector = useChosenConnector(tenant, reach);

  function submit(e: FormEvent) {
    e.preventDefault();
    fill.mutate({ template, ...reach, connector_id: connector || undefined, max_upload_mb: maxUpload },
      { onSuccess: (doc) => onFill(doc as Record<string, unknown>) });
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4 border border-rule bg-surface p-5">
      <h3 className="text-base font-semibold">Start from a template</h3>
      {fill.isError ? <Notice tone="error">{describeError(fill.error)}</Notice> : null}
      <div className="grid gap-4 sm:grid-cols-2">
        <SelectField label="Template" value={template} onChange={(e) => setTemplate(e.target.value)}>
          {templates.data?.map((t) => <option key={t.id} value={t.id}>{t.name}</option>)}
        </SelectField>
        <TextField
          label="Max upload (MB)" type="number" min={1} max={1024} required value={maxUpload}
          onChange={(e) => setMaxUpload(Number(e.target.value))} hint="Largest file the site accepts"
        />
      </div>
      {chosen ? <p className="max-w-prose text-sm text-ink-soft">{chosen.description}</p> : null}
      <ReachFields tenant={tenant} value={reach} onChange={setReach} />
      <div>
        <Button type="submit" busy={fill.isPending} disabled={reach.mode === "connector" && !connector}>Fill the editor</Button>
      </div>
    </form>
  );
}
