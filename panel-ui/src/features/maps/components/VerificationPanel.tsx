import { Button, Notice, formatDate } from "@/shared/ui";
import { ApiError } from "@/shared/api/client";
import { describeError } from "@/shared/api/errors";
import { useVerification, useVerify, type Verification } from "../hooks/useVerification";
import { STILL_OPEN } from "./stillOpen";

const DIFFERENCE: Record<string, string> = {
  status: "answers differently", text: "shows different text", links: "loses links", forms: "loses forms",
  picture: "looks different", refused: "is refused",
};

type Props = { app: string; version: number; canVerify: boolean };

/** What the agent measured on the gateway with this version: does the site still work, are attacks refused. */
export function VerificationPanel({ app, version, canVerify }: Props) {
  const v = useVerification(app, version);
  const verify = useVerify(app, version);
  const none = v.error instanceof ApiError && v.error.status === 404;
  if (v.isError && !none) return <Notice tone="error">{describeError(v.error)}</Notice>;
  const running = v.data && ["queued", "running"].includes(v.data.status);
  return (
    <section aria-labelledby="verify-title" className="flex flex-col gap-4 border border-rule bg-surface p-5">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 id="verify-title" className="text-base font-semibold">Verification</h3>
        {canVerify ? (
          <Button onClick={() => verify.mutate()} busy={verify.isPending} disabled={!!running}>Verify</Button>
        ) : null}
      </div>
      {verify.isError ? <Notice tone="error">{describeError(verify.error)}</Notice> : null}
      {none || !v.data ? (
        <p className="max-w-prose text-sm text-ink-soft">
          Not verified yet. Verify opens every studied page with and without Equalizer using this version, and tries a
          set of attacks on every parameter and field, on the gateway only: nothing is sent to the site.
        </p>
      ) : running ? (
        <p className="text-sm text-ink-soft">The agent is verifying this version…</p>
      ) : (
        <Result r={v.data} />
      )}
    </section>
  );
}

function Result({ r }: { r: Verification }) {
  if (r.status === "failed") return <Notice tone="error">The verification failed: {r.error}</Notice>;
  const s = r.score;
  return (
    <div className="flex flex-col gap-4">
      {s ? (
        <dl className="grid gap-4 sm:grid-cols-3">
          <Figure value={`${s.functions_identical} / ${s.functions_total}`} label="pages identical through Equalizer"
            good={s.functions_identical === s.functions_total} />
          <Figure value={`${s.inputs_protected} / ${s.inputs_total}`} label="inputs that refuse every attack"
            good={s.inputs_protected === s.inputs_total} />
          <Figure value={`${s.area_percent} %`} label="of the pages served only by the site area" />
        </dl>
      ) : null}
      {r.broken?.length ? (
        <List title={`Broken by this version (${r.broken.length})`}
          items={r.broken.map((b) => `${b.path}  ${DIFFERENCE[b.difference] ?? b.difference}${b.detail ? `: ${b.detail}` : ""}`)} />
      ) : (
        <p className="text-sm">No studied page breaks with this version.</p>
      )}
      {r.open?.length ? (
        <List title={`Still open (${r.open.length})`}
          items={r.open.map((o) => `${o.target}  ${STILL_OPEN[o.reason] ?? o.reason}${o.detail ? ` (${o.detail})` : ""}`)} />
      ) : null}
      <p className="text-xs text-ink-soft">
        Verified {r.finished_at ? formatDate(r.finished_at) : ""}{r.requested_by ? ` at the request of ${r.requested_by}` : " after the discovery"}.
        Pages behind a login are not verified yet.
      </p>
    </div>
  );
}

function Figure({ value, label, good }: { value: string; label: string; good?: boolean }) {
  return (
    <div>
      <dd className={`text-2xl font-semibold ${good === false ? "text-state-review" : ""}`}>{value}</dd>
      <dt className="text-sm text-ink-soft">{label}</dt>
    </div>
  );
}

function List({ title, items }: { title: string; items: string[] }) {
  return (
    <details>
      <summary className="cursor-pointer text-sm font-semibold">{title}</summary>
      <ul className="mt-2 max-h-64 overflow-auto font-mono text-[12.5px] leading-relaxed">
        {items.slice(0, 300).map((t, i) => <li key={i}>{t}</li>)}
      </ul>
    </details>
  );
}
