import { parseMap, toEditorYaml, type Parsed } from "./yaml";

/** Larger files are not maps: refused before reading. */
export const MAX_IMPORT = 2 * 1024 * 1024;

/** The YAML file of a version: the map without the meta the panel owns. */
export function exportName(app: string, version: number) {
  return `${app}-map-v${version}.yaml`;
}

export function exportMap(app: string, version: number, doc: Record<string, unknown>) {
  const header = `# Equalizer map of ${app}, version ${version}. Import it as a new draft: it is checked and approved again.\n`;
  const blob = new Blob([header + toEditorYaml(doc)], { type: "application/yaml" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = exportName(app, version);
  a.click();
  URL.revokeObjectURL(url);
}

/** Reads a YAML or JSON map file; JSON is a subset of YAML. */
export async function readMapFile(file: File): Promise<Parsed> {
  if (file.size > MAX_IMPORT) return { ok: false, message: "The file is larger than 2 MB: it is not a map." };
  const parsed = parseMap(await file.text());
  if (!parsed.ok) return parsed;
  const { meta: _meta, ...doc } = parsed.value;
  if (!doc.legacy || !doc.roles) return { ok: false, message: "This file has no legacy or roles section: it is not an Equalizer map." };
  return { ok: true, value: doc };
}
