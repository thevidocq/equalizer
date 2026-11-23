import { Notice, formatDate } from "@/shared/ui";
import { ApiError } from "@/shared/api/client";
import { describeError } from "@/shared/api/errors";
import { Link } from "react-router";
import { useVerification, type Verification } from "../hooks/useVerification";
import { STILL_OPEN } from "./stillOpen";
import { VerifyForm } from "./VerifyForm";

const DIFFERENCE: Record<string, string> = {
  status: "answers differently", text: "shows different text", links: "loses links", forms: "loses forms",
  picture: "looks different", refused: "is refused",
};

type Props = { app: string; version: number; canVerify: boolean; versionLink?: (v: number) => string };

/** What the agent measured on the gateway with this version: does the site still work, are attacks refused. */
export function VerificationPanel({ app, version, canVerify, versionLink = (v) => `../${v}` }: Props) {
  const v = useVerification(app, version);
  const none = v.error instanceof ApiError && v.error.status === 404;
  if (v.isError && !none) return <Notice tone="error">{describeError(v.error)}</Notice>;
  const running = v.data && ["queued", "running"].includes(v.data.status);
  return (
    <section aria-labelledby="verify-title" className="flex flex-col gap-4 border border-rule bg-surface p-5">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 id="verify-title" className="text-base font-semibold">Check on the site</h3>
      </div>
      <p className="max-w-prose text-sm text-ink-soft">
        The agent tries this version on a gateway of its own, against the real site, and changes nothing: it opens every
        page the discovery found with and without Equalizer and compares them, then tries attacks on every parameter and
        field, which the gateway judges without sending them to the site. Discovery finds pages; this checks a version.
      </p>
      {none || !v.data ? (
        <p className="max-w-prose text-sm text-ink-soft">
          Not checked yet: for example a draft changed by hand.
        </p>
      ) : running ? (
        <p className="text-sm text-ink-soft">The agent is checking this version on the site…</p>
      ) : (
        <Result r={v.data} link={versionLink} />
      )}
      {canVerify && !running ? <VerifyForm app={app} version={version} checked={!none && !!v.data} /> : null}
    </section>
  );
}

function Result({ r, link }: { r: Verification; link: (v: number) => string }) {
  if (r.status === "failed") return <Notice tone="error">The check failed: {r.error}</Notice>;
  const s = r.score;
  return (
    <div className="flex flex-col gap-4">
      {s ? (
        <dl className="grid gap-4 sm:grid-cols-3">
          <Figure value={`${s.functions_identical} / ${s.functions_total}`} label="pages identical through Equalizer"
            hint="The site works as before." good={s.functions_identical === s.functions_total} />
          <Figure value={`${s.inputs_protected} / ${s.inputs_total}`} label="inputs that refuse every attack"
            hint="Parameters and form fields; the others are under Still open." good={s.inputs_protected === s.inputs_total} />
          <Figure value={`${s.area_percent} %`} label="of the pages served only by the site area"
            hint="Rebuilt, but without a precise check of their values." />
        </dl>
      ) : null}
      {r.corrected_version ? (
        <Notice tone="info">
          The agent corrected this version into{" "}
          <Link className="font-semibold text-signal hover:underline" to={link(r.corrected_version)} relative="path">
            version {r.corrected_version}</Link>: {r.corrections?.length ?? 0} change(s), listed below. This version keeps its own measure.
        </Notice>
      ) : null}
      {r.corrections?.length ? (
        <List title={`Corrections (${r.corrections.length})`}
          items={r.corrections.map((c) => `${c.entry}  ${c.change}${c.detail ? `: ${c.detail}` : ""}  (${c.reason})`)} />
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
        Checked {r.finished_at ? formatDate(r.finished_at) : ""}{r.requested_by ? ` at the request of ${r.requested_by}` : " after the discovery"}.{" "}
        {r.accounts?.length ? `Pages behind the login of ${r.accounts.join(", ")} checked too.` : "Public pages only: no test account."}
      </p>
    </div>
  );
}

function Figure({ value, label, hint, good }: { value: string; label: string; hint: string; good?: boolean }) {
  return (
    <div>
      <dd className={`text-2xl font-semibold ${good === false ? "text-state-review" : ""}`}>{value}</dd>
      <dt className="text-sm text-ink-soft">{label}</dt>
      <dd className="text-xs text-ink-soft">{hint}</dd>
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
