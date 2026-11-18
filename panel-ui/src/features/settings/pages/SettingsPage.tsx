import { useState, type FormEvent } from "react";
import { useParams } from "react-router";
import { Button, Notice, PageHeader, SelectField, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useLLM, useLLMActions, type LLMInput } from "../hooks/useLLM";

const models: Record<LLMInput["provider"], string> = { anthropic: "claude-haiku-5-5", openai: "gpt-5-mini", local: "llama3.1" };

/** Tenant settings: the AI model used by discovery. */
export function SettingsPage() {
  const tenant = useParams().tenant ?? "";
  const llm = useLLM(tenant);
  if (!llm.data && !llm.isError) return null;
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="Settings" />
      {llm.isError ? <Notice tone="error">{describeError(llm.error)}</Notice> : null}
      {llm.data ? <LLMForm key={llm.data.provider ?? "none"} tenant={tenant} current={llm.data} /> : null}
    </div>
  );
}

type Current = { provider?: string | null; model?: string | null; endpoint?: string | null; has_api_key: boolean };

function LLMForm({ tenant, current }: { tenant: string; current: Current }) {
  const { save, remove } = useLLMActions(tenant);
  const toast = useToast();
  const [provider, setProvider] = useState<string>(current.provider ?? "");
  const [model, setModel] = useState(current.model ?? "");
  const [endpoint, setEndpoint] = useState(current.endpoint ?? "");
  const [key, setKey] = useState("");

  function submit(e: FormEvent) {
    e.preventDefault();
    if (!provider) {
      remove.mutate(undefined, { onSuccess: () => toast("AI turned off: discovery runs without it") });
      return;
    }
    const p = provider as LLMInput["provider"];
    save.mutate({ provider: p, model: model || models[p], endpoint: p === "local" ? endpoint : null, api_key: key || undefined },
      { onSuccess: () => { setKey(""); toast("AI model saved"); } });
  }
  const error = save.error ?? remove.error;

  return (
    <form onSubmit={submit} className="flex max-w-xl flex-col gap-4">
      <div>
        <h2 className="text-base font-semibold">AI for discovery</h2>
        <p className="text-sm text-ink-soft">
          Your own model and key: the platform has none and never sees the key. Discovery sends it the structure of the site
          (paths, parameter names, form fields) with values masked. Without a model, discovery still runs.
        </p>
      </div>
      {error ? <Notice tone="error">{describeError(error)}</Notice> : null}
      <SelectField label="Provider" value={provider} onChange={(e) => { setProvider(e.target.value); setModel(""); }}>
        <option value="">No AI</option>
        <option value="anthropic">Claude (Anthropic)</option>
        <option value="openai">OpenAI</option>
        <option value="local">Own server (OpenAI-compatible: Ollama, vLLM)</option>
      </SelectField>
      {provider ? (
        <>
          <TextField label="Model" value={model} placeholder={models[provider as LLMInput["provider"]]} onChange={(e) => setModel(e.target.value)} />
          {provider === "local" ? (
            <TextField label="Server address" type="url" required value={endpoint} placeholder="https://ai.example.org/v1"
              onChange={(e) => setEndpoint(e.target.value)} hint="Reachable from the Internet over HTTPS." />
          ) : null}
          <TextField label="API key" type="password" autoComplete="off" value={key} onChange={(e) => setKey(e.target.value)}
            placeholder={current.has_api_key ? "Saved: leave empty to keep it" : ""}
            required={provider !== "local" && !current.has_api_key} />
        </>
      ) : null}
      <div>
        <Button type="submit" variant="primary" busy={save.isPending || remove.isPending}>Save</Button>
      </div>
    </form>
  );
}
