import Markdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import { Link } from "react-router";
import type { Topic } from "../guide";

const md: Components = {
  p: ({ children }) => <p className="max-w-prose leading-relaxed">{children}</p>,
  ul: ({ children }) => <ul className="ml-5 flex max-w-prose list-disc flex-col gap-1.5">{children}</ul>,
  ol: ({ children }) => <ol className="ml-5 flex max-w-prose list-decimal flex-col gap-1.5">{children}</ol>,
  a: ({ href, children }) => <a href={href} className="text-signal underline" target="_blank" rel="noreferrer">{children}</a>,
  img: ({ src, alt }) => (
    <img src={typeof src === "string" ? src : undefined} alt={alt ?? ""} loading="lazy"
      className="w-full max-w-4xl rounded-[var(--radius-panel)] border border-rule" />
  ),
  code: ({ children, className }) =>
    className ? <code className="font-mono">{children}</code>
      : <code className="rounded bg-rule-soft px-1 py-0.5 font-mono text-[0.88em]">{children}</code>,
  pre: ({ children }) => (
    <pre className="max-w-4xl overflow-x-auto rounded-[var(--radius-control)] bg-ink p-3 font-mono text-[12.5px] leading-relaxed text-paper">{children}</pre>
  ),
  table: ({ children }) => (
    <div className="max-w-4xl overflow-x-auto"><table className="w-full border-collapse text-sm">{children}</table></div>
  ),
  th: ({ children }) => <th className="border-b border-rule px-2 py-1.5 text-left font-semibold">{children}</th>,
  td: ({ children }) => <td className="border-b border-rule-soft px-2 py-1.5 align-top">{children}</td>,
};

type Props = { topic: Topic; prev?: Topic; next?: Topic };

/** One topic of the guide, with the way to the topics around it. */
export function HelpArticle({ topic, prev, next }: Props) {
  return (
    <article className="flex min-w-0 flex-col gap-4">
      <header>
        <p className="text-sm text-ink-soft">{topic.macroTitle}</p>
        <h2 className="text-[1.45rem] font-bold leading-tight">{topic.title}</h2>
      </header>
      <Markdown remarkPlugins={[remarkGfm]} components={md}>{topic.body}</Markdown>
      <nav aria-label="Topics around this one" className="mt-6 flex justify-between gap-4 border-t border-rule pt-4 text-sm">
        {prev ? <Link to={`/help/${prev.id}`} className="text-signal hover:underline">Previous: {prev.title}</Link> : <span />}
        {next ? <Link to={`/help/${next.id}`} className="text-right text-signal hover:underline">Next: {next.title}</Link> : <span />}
      </nav>
    </article>
  );
}
