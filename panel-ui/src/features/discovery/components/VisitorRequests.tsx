import { Link } from "react-router";
import { Button, Notice } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useVerification } from "@/features/maps";
import { useAdopt, type AdoptItem } from "../hooks/useAdopt";

type Item = { label: string; clients: number; why: string; adopt: AdoptItem };
type R = Record<string, any>; // eslint-disable-line @typescript-eslint/no-explicit-any

// Reports of agents before 0.2.11 listed requests unchecked, each with a
// verdict: left out (the panel refuses to add them too).
const actionable = (i: R) => Boolean(i.add) && i.verdict === undefined;

function Row({ item, canAdd, onAdd, busy }: { item: Item; canAdd: boolean; onAdd: (a: AdoptItem) => void; busy: boolean }) {
  return (
    <li className="flex flex-wrap items-start justify-between gap-2 border-b border-rule py-2">
      <div className="flex min-w-0 flex-col">
        <span className="break-all font-mono text-[12.5px]">{item.label}</span>
        <span className="text-xs text-ink-soft">{item.why}</span>
      </div>
      {canAdd ? <Button busy={busy} onClick={() => onAdd(item.adopt)}>Add to a new draft</Button> : null}
    </li>
  );
}

function Group({ title, items, canAdd, onAdd, busy }: { title: string; items: Item[]; canAdd: boolean;
  onAdd: (a: AdoptItem) => void; busy: boolean }) {
  if (!items.length) return null;
  return (
    <section className="flex flex-col gap-1">
      <h3 className="text-sm font-semibold">{title} ({items.length})</h3>
      <ul>{items.map((i, n) => <Row key={n} item={i} canAdd={canAdd} onAdd={onAdd} busy={busy} />)}</ul>
    </section>
  );
}

/**
 * What visitors asked for and the map refuses, only as actions: the agent
 * lists an item once several visitors asked for it and it checked it on the
 * site (a real file, a key that changes the page). It never adds one itself
 * (anyone can ask for anything); a person does, in one click, into a new
 * draft to review and publish.
 */
export function VisitorRequests({ app, discovery, draft, report, canAdd }: {
  app: string; discovery: number; draft: number | null; report: R; canAdd: boolean;
}) {
  const adopt = useAdopt(app, discovery);
  const check = useVerification(app, draft ?? 0, draft != null);
  // while the agent checks its draft, a change would land on a version about to be replaced
  const checking = draft != null && (check.isPending || ["queued", "running"].includes(check.data?.status ?? ""));
  const files: Item[] = (report.files_refused ?? []).filter(actionable).map((f: R) => ({
    label: f.path, clients: f.clients, why: f.why ?? "", adopt: { kind: "file", path: f.path },
  }));
  const keys: Item[] = (report.query_dropped ?? []).filter(actionable).map((k: R) => ({
    label: `${k.route}  ?${k.key}=`, clients: k.clients, why: k.why ?? "", adopt: { kind: "key", route: k.route, key: k.key },
  }));
  if (!files.length && !keys.length) return null;
  return (
    <div className="flex flex-col gap-4">
      {adopt.isSuccess ? (
        <Notice tone="info">
          Added in <Link className="font-semibold text-signal hover:underline" to={`../maps/${adopt.data.version}`}>
            version {adopt.data.version}</Link>: review it and publish it.
        </Notice>
      ) : null}
      {adopt.isError ? <Notice tone="error">{describeError(adopt.error)}</Notice> : null}
      {canAdd && checking ? <p className="text-xs text-ink-soft">You can add these once the agent has checked its draft.</p> : null}
      <Group title="Files visitors asked for that the map refuses" items={files} canAdd={canAdd && !checking}
        onAdd={(a) => adopt.mutate(a)} busy={adopt.isPending} />
      <Group title="Page options visitors use that the map drops" items={keys} canAdd={canAdd && !checking}
        onAdd={(a) => adopt.mutate(a)} busy={adopt.isPending} />
    </div>
  );
}
