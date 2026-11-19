import { useMemo, useState } from "react";
import { Navigate, useParams } from "react-router";
import { PageHeader } from "@/shared/ui";
import { useGuide } from "../hooks/useGuide";
import { search } from "../guide";
import { HelpNav } from "../components/HelpNav";
import { HelpArticle } from "../components/HelpArticle";
import { HelpResults } from "../components/HelpResults";

/** The guide for this kind of user: topics on the left, search on top. */
export function HelpPage() {
  const guide = useGuide();
  const { topic: id } = useParams();
  const [query, setQuery] = useState("");
  const hits = useMemo(() => (guide ? search(guide, query) : []), [guide, query]);
  if (!guide) return null;
  const at = guide.topics.findIndex((t) => t.id === id);
  if (at < 0) return <Navigate to={`/help/${guide.topics[0]?.id ?? ""}`} replace />;
  const searching = query.trim().length > 1;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="Help" subtitle={guide.title} />
      <div className="grid gap-8 lg:grid-cols-[16rem_1fr]">
        <aside className="flex flex-col gap-5 lg:sticky lg:top-6 lg:max-h-[calc(100vh-3rem)] lg:self-start lg:overflow-y-auto">
          <label className="flex flex-col gap-1.5 text-sm font-medium">
            Search the guide
            <input
              type="search" value={query} onChange={(e) => setQuery(e.target.value)} placeholder="e.g. upload, logs, rollback"
              className="rounded-[var(--radius-control)] border border-rule bg-surface px-3 py-2 font-normal"
            />
          </label>
          <HelpNav guide={guide} />
        </aside>
        <div className="min-w-0">
          {searching ? (
            <HelpResults query={query.trim()} hits={hits} onOpen={() => setQuery("")} />
          ) : (
            <HelpArticle topic={guide.topics[at]!} prev={guide.topics[at - 1]} next={guide.topics[at + 1]} />
          )}
        </div>
      </div>
    </div>
  );
}
