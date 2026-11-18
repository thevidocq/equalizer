import { useNavigate, useParams, useSearchParams } from "react-router";
import { useToast } from "@/shared/ui";
import { useCreateDraft, useMapVersion } from "../hooks/useMaps";
import { MapEditor } from "../components/MapEditor";
import { toEditorYaml } from "../components/yaml";

/** A new draft, empty or starting from an existing version (?from=N). */
export function NewVersionPage() {
  const { app = "" } = useParams();
  const from = Number(useSearchParams()[0].get("from") ?? 0);
  const source = useMapVersion(app, from);
  const create = useCreateDraft(app);
  const navigate = useNavigate();
  const toast = useToast();

  if (from > 0 && !source.data) return null;
  return (
    <div className="flex flex-col gap-4">
      <h2 className="text-lg font-semibold">{from > 0 ? `New version from version ${from}` : "New version"}</h2>
      <MapEditor
        key={from}
        initial={toEditorYaml(source.data?.document as Record<string, unknown> | undefined)}
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
