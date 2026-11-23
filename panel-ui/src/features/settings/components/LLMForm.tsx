import { useEffect, useState, type FormEvent } from "react";
import { Button, Notice, SelectField, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useLLMActions, useLLMModels, type LLMInput } from "../hooks/useLLM";

const fallback: Record<LLMInput["provider"], string> = { anthropic: "claude-haiku-5-5", openai: "gpt-5-mini", local: "llama3.1" };

type Current = { provider?: string | null; model?: string | null; endpoint?: string | null; has_api_key: boolean };

/**
 * The tenant's AI model. Once the provider and the key are known, the
 * provider's own list of models is offered; the name is typed only when the
 * provider cannot be asked.
 */
export function LLMForm({ tenant, current }: { tenant: string; current: Current }) {
  const { save, remove } = useLLMActions(tenant);
  const models = useLLMModels(tenant);
  const toast = useToast();
  const [provider, setProvider] = useState<string>(current.provider ?? "");
  const [model, setModel] = useState(current.model ?? "");
  const [endpoint, setEndpoint] = useState(current.endpoint ?? "");
  const [key, setKey] = useState("");
  const p = provider as LLMInput["provider"];
  const canAsk = !!provider && (key !== "" || (current.has_api_key && current.provider === provider) || (p === "local" && endpoint !== ""));

  function ask() {
    if (!canAsk) return;
    models.mutate({ provider: p, endpoint: p === "local" ? endpoint : null, api_key: key || undefined }, {
      onSuccess: (list) => { if (list.length > 0 && !list.some((m) => m.id === model)) setModel(list[0]!.id); },
    });
  }
  // A saved key: the list is there when the page opens.
  useEffect(() => { if (current.has_api_key && current.provider) ask(); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  function submit(e: FormEvent) {
    e.preventDefault();
    if (!provider) {
      remove.mutate(undefined, { onSuccess: () => toast("AI turned off: discovery runs without it") });
      return;
    }
    save.mutate({ provider: p, model: model || fallback[p], endpoint: p === "local" ? endpoint : null, api_key: key || undefined },
      { onSuccess: () => { setKey(""); toast("AI model saved"); } });
  }
  const error = save.error ?? remove.error;
  const list = models.data ?? [];

  return (
    <form onSubmit={submit} className="flex max-w-xl flex-col gap-4">
      <div>
        <h2 className="text-base font-semibold">AI model</h2>
        <p className="text-sm text-ink-soft">
          Your own model and key, used by discovery, the review of a version before you publish it, and each site's
          assistant. The platform has none and never shows the key. Discovery sends it the structure of the site (paths,
          parameter names, form fields) with values masked. Without a model discovery still runs; the assistant needs one.
        </p>
      </div>
      {error ? <Notice tone="error">{describeError(error)}</Notice> : null}
      <SelectField label="Provider" value={provider} onChange={(e) => { setProvider(e.target.value); setModel(""); models.reset(); }}>
        <option value="">No AI</option>
        <option value="anthropic">Claude (Anthropic)</option>
        <option value="openai">OpenAI</option>
        <option value="local">Own server (OpenAI-compatible: Ollama, vLLM)</option>
      </SelectField>
      {provider ? (
        <>
          {provider === "local" ? (
            <TextField label="Server address" type="url" required value={endpoint} placeholder="https://ai.example.org/v1"
              onChange={(e) => setEndpoint(e.target.value)} onBlur={ask} hint="Reachable from the Internet over HTTPS." />
          ) : null}
          <TextField label="API key" type="password" autoComplete="off" value={key} onChange={(e) => setKey(e.target.value)} onBlur={ask}
            placeholder={current.has_api_key && current.provider === provider ? "Saved: leave empty to keep it" : ""}
            required={provider !== "local" && !(current.has_api_key && current.provider === provider)} />
          {list.length > 0 ? (
            <SelectField label="Model" value={model} onChange={(e) => setModel(e.target.value)}>
              {list.map((m) => <option key={m.id} value={m.id}>{m.name === m.id ? m.id : `${m.name} (${m.id})`}</option>)}
            </SelectField>
          ) : (
            <TextField label="Model" value={model} placeholder={fallback[p]} onChange={(e) => setModel(e.target.value)}
              hint={models.isPending ? "Asking the provider for its models…" : models.isError
                ? `${describeError(models.error)}.` : "Enter the key: the provider's models appear here."} />
          )}
        </>
      ) : null}
      <div className="flex flex-wrap gap-2">
        <Button type="submit" variant="primary" busy={save.isPending || remove.isPending}>Save</Button>
        {provider && canAsk ? <Button onClick={ask} busy={models.isPending}>Refresh the models</Button> : null}
      </div>
    </form>
  );
}
