import { Link } from "react-router";
import { Button, Notice } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useAdopt, type AdoptItem } from "../hooks/useAdopt";

type Item = { label: string; clients: number; verdict: "site" | "suspect"; why: string; adopt: AdoptItem | null };
type R = Record<string, any>; // eslint-disable-line @typescript-eslint/no-explicit-any

function Row({ item, canAdd, onAdd, busy }: { item: Item; canAdd: boolean; onAdd: (a: AdoptItem) => void; busy: boolean }) {
  return (
    <li className="flex flex-wrap items-start justify-between gap-2 border-b border-rule py-2">
      <div className="flex min-w-0 flex-col">
        <span className="break-all font-mono text-[12.5px]">{item.label}</span>
        <span className="text-xs text-ink-soft">
          {item.clients} {item.clients === 1 ? "visitor" : "visitors"} · {item.why}
        </span>
      </div>
      {canAdd && item.adopt ? (
        <Button busy={busy} onClick={() => item.adopt && onAdd(item.adopt)}>Add to a new draft</Button>
      ) : null}
    </li>
  );
}

function Group({ title, items, canAdd, onAdd, busy }: { title: string; items: Item[]; canAdd: boolean;
  onAdd: (a: AdoptItem) => void; busy: boolean }) {
  const site = items.filter((i) => i.verdict === "site");
  const suspect = items.filter((i) => i.verdict !== "site");
  if (!items.length) return null;
  return (
    <section className="flex flex-col gap-1">
      <h3 className="text-sm font-semibold">{title} ({items.length})</h3>
      {site.length ? (
        <ul>{site.map((i, n) => <Row key={n} item={i} canAdd={canAdd} onAdd={onAdd} busy={busy} />)}</ul>
      ) : null}
      {suspect.length ? (
        <details className="text-sm">
          <summary className="cursor-pointer text-ink-soft">
            {suspect.length} not linked by the site: probably leftovers or someone trying addresses
          </summary>
          <ul>{suspect.map((i, n) => <Row key={n} item={i} canAdd={canAdd} onAdd={onAdd} busy={busy} />)}</ul>
        </details>
      ) : null}
    </section>
  );
}

/**
 * What visitors asked for and the map refuses: never added by the agent
 * (anyone can ask for anything). The agent says whether the site itself uses
 * it; a person adds it in one click, into a new draft to review and publish.
 */
export function VisitorRequests({ app, discovery, report, canAdd }: { app: string; discovery: number; report: R; canAdd: boolean }) {
  const adopt = useAdopt(app, discovery);
  const files: Item[] = (report.files_refused ?? []).map((f: R) => ({
    label: f.path, clients: f.clients, verdict: f.verdict, why: f.why ?? "",
    adopt: f.add ? { kind: "file", path: f.path } : null,
  }));
  const keys: Item[] = (report.query_dropped ?? []).map((k: R) => ({
    label: `${k.route}  ?${k.key}=`, clients: k.clients, verdict: k.verdict, why: k.why ?? "",
    adopt: k.add ? { kind: "key", route: k.route, key: k.key } : null,
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
      <Group title="Files visitors asked for that the map refuses" items={files} canAdd={canAdd}
        onAdd={(a) => adopt.mutate(a)} busy={adopt.isPending} />
      <Group title="Page options visitors use that the map drops" items={keys} canAdd={canAdd}
        onAdd={(a) => adopt.mutate(a)} busy={adopt.isPending} />
    </div>
  );
}
