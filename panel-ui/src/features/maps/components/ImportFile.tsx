import { useId, useState, type ChangeEvent } from "react";
import { Notice } from "@/shared/ui";
import { readMapFile } from "./mapFile";

/** Loads a map file into the editor; nothing is saved until the draft is. */
export function ImportFile({ onLoad }: { onLoad: (doc: Record<string, unknown>, name: string) => void }) {
  const id = useId();
  const [error, setError] = useState("");
  async function pick(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    const r = await readMapFile(file);
    if (!r.ok) {
      setError(r.message);
      return;
    }
    setError("");
    onLoad(r.value, file.name);
  }
  return (
    <div className="flex flex-col gap-2 border border-rule bg-surface p-5">
      <h3 className="text-base font-semibold">Import a map file</h3>
      <p className="text-sm text-ink-soft">
        A YAML or JSON map exported from this or another site. It opens in the editor: tenant, site and domain are set
        from this site, and the draft gets the same checks and approval as any other.
      </p>
      {error ? <Notice tone="error">{error}</Notice> : null}
      <div>
        <label htmlFor={id} className="inline-flex cursor-pointer items-center rounded-[var(--radius-control)] border border-rule bg-surface px-3.5 py-2 text-sm font-semibold hover:border-ink-soft focus-within:outline focus-within:outline-2 focus-within:outline-signal">
          Choose a file
          <input id={id} type="file" accept=".yaml,.yml,.json,application/json,application/yaml" className="sr-only" onChange={pick} />
        </label>
      </div>
    </div>
  );
}
