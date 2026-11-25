import { Menu, Notice } from "@/shared/ui";
import { exportJSON, exportMap } from "./mapFile";
import { useMapExport } from "../hooks/useMapExport";

/** The version as a file: the map itself (YAML, JSON), or written for another tool. */
export function DownloadMenu({ app, version, doc }: { app: string; version: number; doc: Record<string, unknown> }) {
  const other = useMapExport(app, version);
  return (
    <div className="flex flex-col gap-2">
      <Menu label="Download" items={[
        { label: "YAML", note: "The map: import it again as a draft", onSelect: () => exportMap(app, version, doc) },
        { label: "JSON", note: "The same map, as JSON", onSelect: () => exportJSON(app, version, doc) },
        { label: "OpenAPI 3", note: "What the map accepts, for API tools", onSelect: () => other.mutate("openapi") },
        { label: "ModSecurity / Coraza", note: "Rules for a WAF: checks, no rebuilding", onSelect: () => other.mutate("modsecurity") },
      ]} />
      {other.isError ? <Notice tone="error">{(other.error as Error).message}</Notice> : null}
    </div>
  );
}
