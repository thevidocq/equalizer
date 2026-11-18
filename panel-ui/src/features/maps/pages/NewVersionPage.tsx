import { useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { useToast } from "@/shared/ui";
import { useApp } from "@/features/sites";
import { useCreateDraft, useMapVersion } from "../hooks/useMaps";
import { MapEditor } from "../components/MapEditor";
import { TemplatePicker } from "../components/TemplatePicker";
import { toEditorYaml } from "../components/yaml";

/** A new draft: empty, from a template, or from an existing version (?from=N). */
export function NewVersionPage() {
  const { tenant = "", app = "" } = useParams();
  const from = Number(useSearchParams()[0].get("from") ?? 0);
  const source = useMapVersion(app, from);
  const site = useApp(app);
  const create = useCreateDraft(app);
  const navigate = useNavigate();
  const toast = useToast();
  const [filled, setFilled] = useState<{ doc: Record<string, unknown>; n: number } | null>(null);

  if (from > 0 && !source.data) return null;
  const initial = filled?.doc ?? (source.data?.document as Record<string, unknown> | undefined);
  return (
    <div className="flex flex-col gap-4">
      <h2 className="text-lg font-semibold">{from > 0 ? `New version from version ${from}` : "New version"}</h2>
      {from === 0 ? (
        <TemplatePicker tenant={tenant} app={app} kind={site.data?.kind}
          onFill={(doc) => { setFilled((f) => ({ doc, n: (f?.n ?? 0) + 1 })); toast("Template loaded: review it, then save the draft"); }} />
      ) : null}
      <MapEditor
        key={`${from}-${filled?.n ?? 0}`}
        initial={toEditorYaml(initial)}
        busy={create.isPending}
        error={create.error}
        onSave={(document, note) =>
          create.mutate({ document, note }, {
            onSuccess: (v) => { toast(`Draft ${v.version} saved`); navigate(`../${v.version}`, { relative: "path" }); },
          })
        }
      />
    </div>
  );
}
