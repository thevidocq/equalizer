import { Button, Notice, Spinner, formatDate, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { reading, useSourceActions, type SourceReader } from "../hooks/useSource";

/** The last read and the button that reads now (it tests the settings too). */
export function LastRead({ app, s, isAdmin }: { app: string; s: SourceReader; isAdmin: boolean }) {
  const { read } = useSourceActions(app);
  const toast = useToast();
  const r = s.last_read;
  const c = r?.counts;
  return (
    <section className="flex flex-col gap-3">
      <h2 className="text-base font-semibold">Last read</h2>
      {!r ? <p className="text-sm text-ink-soft">No read yet.</p> : null}
      {r && reading(s) ? <Spinner label={r.status === "queued" ? "Waiting for the reader…" : "Reading the files…"} /> : null}
      {r?.status === "failed" ? <Notice tone="error">The read of {formatDate(r.created_at)} failed: {r.error}</Notice> : null}
      {r?.status === "done" && c ? (
        <p className="text-sm">
          {formatDate(r.finished_at ?? r.created_at)}: {c.files} code files ({c.stack}), {c.routes} entry points, {c.inputs} request keys,{" "}
          {c.risks} risky calls, {c.components} components. Discoveries that read the source use it.
        </p>
      ) : null}
      {read.isError ? <Notice tone="error">{describeError(read.error)}</Notice> : null}
      {isAdmin ? (
        <div>
          <Button busy={read.isPending} disabled={s.status !== "ready" || reading(s)}
            onClick={() => read.mutate(undefined, { onSuccess: () => toast("Read queued") })}>
            Read now
          </Button>
        </div>
      ) : null}
    </section>
  );
}
