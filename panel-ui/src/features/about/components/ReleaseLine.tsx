import { useState } from "react";
import { Dialog } from "@/shared/ui";
import { useAbout } from "../hooks/useAbout";

const notes = (release: string) => `https://github.com/thevidocq/equalizer/releases/tag/v${release}`;

/**
 * The running release, small at the bottom of the menu; it opens the
 * details: the release notes and, for platform administrators, the release
 * each service runs.
 */
export function ReleaseLine() {
  const about = useAbout().data;
  const [open, setOpen] = useState(false);
  if (!about) return null;
  const released = /^\d+\.\d+\.\d+$/.test(about.release);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}
        className="mt-1 w-full rounded-[var(--radius-control)] px-2 py-1 text-left text-[11px] text-paper/45 hover:bg-paper/8 hover:text-paper/75">
        Equalizer {about.release}
      </button>
      <Dialog open={open} onOpenChange={setOpen} title={`Equalizer ${about.release}`}
        description={released ? "The release running on this platform." : "A development build, not a release."}>
        <div className="flex flex-col gap-4 text-sm">
          {about.services?.length ? (
            <dl className="grid grid-cols-[max-content_1fr] gap-x-6 gap-y-1">
              {about.services.map((s) => (
                <div key={s.name} className="contents">
                  <dt className="text-ink-soft">{s.name}</dt>
                  <dd className={s.releases.length === 1 && s.releases[0] === about.release ? "" : "text-state-review"}>
                    {s.releases.length ? s.releases.join(", ") : "does not answer"}
                    {s.releases.length > 1 ? " (update in progress)" : ""}
                  </dd>
                </div>
              ))}
            </dl>
          ) : null}
          {released ? (
            <a className="font-medium text-signal hover:underline" href={notes(about.release)} target="_blank" rel="noreferrer">
              Release notes
            </a>
          ) : null}
        </div>
      </Dialog>
    </>
  );
}
