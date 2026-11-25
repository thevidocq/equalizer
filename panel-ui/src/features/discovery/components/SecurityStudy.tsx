import { Disclosure, Notice } from "@/shared/ui";
import { ACTION, COVERAGE, SEVERITY, surfaceLine, type Finding, type Study } from "./securityStudy";

function Severity({ s }: { s: Finding["severity"] }) {
  const v = SEVERITY[s];
  return (
    <span className="inline-flex shrink-0 items-center gap-1.5 text-[13px] font-medium">
      <span aria-hidden="true" className="size-2 rounded-full" style={{ background: v.color }} />
      {v.word}
    </span>
  );
}

function Row({ label, text }: { label: string; text?: string }) {
  if (!text) return null;
  return <p className="text-sm"><span className="font-semibold">{label}: </span>{text}</p>;
}

function FindingItem({ f }: { f: Finding }) {
  return (
    <Disclosure title={<span className="inline-flex items-center gap-3"><Severity s={f.severity} />{f.title}</span>}
      note={`· ${COVERAGE[f.coverage]}`}>
      <div className="flex flex-col gap-1.5">
        <Row label="Why" text={f.why} />
        <Row label="What an attacker would try" text={f.attack} />
        <Row label="The map" text={f.coverage_why} />
        <Row label="Next step" text={f.action_detail ? `${ACTION[f.action]}: ${f.action_detail}` : ACTION[f.action]} />
        <ul className="font-mono text-[12.5px] leading-relaxed text-ink-soft">
          {f.refs.map((r) => <li key={r.id}>{r.label}</li>)}
        </ul>
      </div>
    </Disclosure>
  );
}

/** The AI's security study: the discovery's conclusion. */
export function SecurityStudy({ study }: { study?: Study }) {
  if (!study) return null;
  if (study.pending) {
    return (
      <Notice tone="info">
        The security study is written after the agent's check of the draft, on the map as the check leaves it: it
        appears here then.
      </Notice>
    );
  }
  if (!study.used) {
    return study.error
      ? <Notice tone="error">The AI security study did not run: {study.error}.</Notice>
      : <Notice tone="warning">No AI security study{study.reason ? `: ${study.reason}` : ""}.</Notice>;
  }
  const findings = study.findings ?? [];
  const counts = (Object.keys(SEVERITY) as Finding["severity"][]).filter((s) => study.counts?.[s]);
  return (
    <section className="flex flex-col gap-3 rounded-[var(--radius-panel)] border border-rule bg-surface p-4">
      <h3 className="text-base font-semibold">Security study{study.version ? <span className="font-normal text-ink-soft"> · on version {study.version}</span> : null}</h3>
      {study.summary ? <p className="text-sm leading-relaxed">{study.summary}</p> : null}
      <p className="text-[13px] text-ink-soft">{surfaceLine(study.surface)}</p>
      {counts.length ? (
        <div className="flex flex-wrap gap-4">
          {counts.map((s) => <span key={s} className="inline-flex items-center gap-1.5"><Severity s={s} /><span className="text-[13px]">{study.counts?.[s]}</span></span>)}
        </div>
      ) : <p className="text-sm">No finding: nothing the AI saw needs more than the map.</p>}
      {study.partial ? <Notice tone="warning">A part of the study got no answer from the AI: run the discovery again for the whole of it.</Notice> : null}
      {study.steps?.length ? (
        <div>
          <h4 className="text-sm font-semibold">First steps</h4>
          <ol className="list-decimal pl-5 text-sm leading-relaxed">{study.steps.map((s, i) => <li key={i}>{s}</li>)}</ol>
        </div>
      ) : null}
      {findings.length ? <div className="flex flex-col gap-2">{findings.map((f, i) => <FindingItem key={i} f={f} />)}</div> : null}
      <p className="text-[12.5px] text-ink-soft">
        Written by the AI from names, types and places only, never values or code; it changes nothing in the draft.
        What it recalls about a version is to be verified.
      </p>
    </section>
  );
}
