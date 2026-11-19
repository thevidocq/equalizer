import { useState, type FormEvent } from "react";
import { Button, Notice, SelectField, TextField } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useConnectors } from "@/features/connectors";
import { useFillTemplate, useTemplates, type TemplateInput } from "../hooks/useTemplates";

type Props = { tenant: string; app: string; kind: string | null | undefined; onFill: (doc: Record<string, unknown>) => void };

/** Fills the editor with a starting map for the site's CMS. */
export function TemplatePicker({ tenant, app, kind, onFill }: Props) {
  const templates = useTemplates();
  const connectors = useConnectors(tenant);
  const fill = useFillTemplate(app);
  const [input, setInput] = useState<TemplateInput>({ template: kind ?? "generic", connector_id: "", address: "127.0.0.1:8443", scheme: "https", max_upload_mb: 50 });
  const set = (k: keyof TemplateInput) => (e: { target: { value: string } }) => setInput({ ...input, [k]: e.target.value });
  const chosen = templates.data?.find((t) => t.id === input.template);
  const connector = input.connector_id || connectors.data?.[0]?.id || "";

  function submit(e: FormEvent) {
    e.preventDefault();
    fill.mutate({ ...input, connector_id: connector }, { onSuccess: (doc) => onFill(doc as Record<string, unknown>) });
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4 border border-rule bg-surface p-5">
      <h3 className="text-base font-semibold">Start from a template</h3>
      {fill.isError ? <Notice tone="error">{describeError(fill.error)}</Notice> : null}
      {connectors.data?.length === 0 ? <Notice tone="info">Add a connector first (Connectors in the sidebar): the map needs it.</Notice> : null}
      <div className="grid gap-4 sm:grid-cols-5">
        <SelectField label="Template" value={input.template} onChange={set("template")}>
          {templates.data?.map((t) => <option key={t.id} value={t.id}>{t.name}</option>)}
        </SelectField>
        <SelectField label="Connector" value={connector} onChange={set("connector_id")}>
          {connectors.data?.map((c) => <option key={c.id} value={c.id}>{c.id}</option>)}
        </SelectField>
        <TextField label="Address on the server" value={input.address} onChange={set("address")} hint="host:port the connector reaches" />
        <SelectField label="Protocol" value={input.scheme} onChange={set("scheme")}>
          <option value="https">HTTPS</option><option value="http">HTTP</option>
        </SelectField>
        <TextField
          label="Max upload (MB)" type="number" min={1} max={1024} required value={input.max_upload_mb}
          onChange={(e) => setInput({ ...input, max_upload_mb: Number(e.target.value) })} hint="Largest file the site accepts"
        />
      </div>
      {chosen ? <p className="max-w-prose text-sm text-ink-soft">{chosen.description}</p> : null}
      <div>
        <Button type="submit" busy={fill.isPending} disabled={!connector}>Fill the editor</Button>
      </div>
    </form>
  );
}
