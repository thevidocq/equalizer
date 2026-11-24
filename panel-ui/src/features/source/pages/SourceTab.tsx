import { useParams } from "react-router";
import { hasRole, useMe } from "@/features/auth";
import { describeError } from "@/shared/api/errors";
import { Button, Notice, useToast } from "@/shared/ui";
import { useSource, useSourceActions } from "../hooks/useSource";
import { SourceForm } from "../components/SourceForm";
import { ReaderStatus } from "../components/ReaderStatus";
import { LastRead } from "../components/LastRead";

/**
 * The site's source reader: an optional container next to the connector
 * that reads the site's code, read only, and sends the panel a summary of
 * it (entry points, types, risky calls, components). Never the code.
 */
export function SourceTab() {
  const { tenant = "", app = "" } = useParams();
  const me = useMe().data;
  const src = useSource(app);
  const { remove } = useSourceActions(app);
  const toast = useToast();
  const isAdmin = hasRole(me, tenant, "admin");
  if (src.isError) return <Notice tone="error">{describeError(src.error)}</Notice>;
  if (!src.data) return null;
  const s = src.data;
  return (
    <div className="flex max-w-3xl flex-col gap-8">
      <p className="max-w-prose text-sm text-ink-soft">
        Optional. The source reader runs next to the connector, never on the web server. It reads the site's code files, read
        only, and sends Equalizer a summary: the entry points, the type the code gives each field, the calls where a value may do
        harm, the components and their versions. The code, the values and the secrets never leave your network. Discoveries
        that read the source give the map's fields the code's types and report what the links do not show.
      </p>
      {isAdmin ? <SourceForm key={`${s.method}-${s.host}-${s.path}`} app={app} s={s} /> : null}
      {s.configured || !isAdmin ? <ReaderStatus app={app} s={s} isAdmin={isAdmin} /> : null}
      {s.configured ? <LastRead app={app} s={s} isAdmin={isAdmin} /> : null}
      {isAdmin && s.configured ? (
        <div>
          {remove.isError ? <Notice tone="error">{describeError(remove.error)}</Notice> : null}
          <Button variant="danger" busy={remove.isPending} onClick={() => remove.mutate(undefined, { onSuccess: () => toast("Source reader removed: its token stopped") })}>
            Remove the reader
          </Button>
        </div>
      ) : null}
    </div>
  );
}
