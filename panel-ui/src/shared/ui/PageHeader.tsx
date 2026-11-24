import type { ReactNode } from "react";

type Props = { title: string; titleAction?: ReactNode; subtitle?: ReactNode; actions?: ReactNode };

/** A page's title, with an optional small action next to it (an icon), a subtitle and buttons. */
export function PageHeader({ title, titleAction, subtitle, actions }: Props) {
  return (
    <header className="flex flex-wrap items-end justify-between gap-4">
      <div>
        <div className="flex items-center gap-1.5">
          <h1 className="text-[1.6rem] font-bold leading-tight tracking-[-0.01em]">{title}</h1>
          {titleAction}
        </div>
        {subtitle ? <p className="mt-1 text-ink-soft">{subtitle}</p> : null}
      </div>
      {actions ? <div className="flex gap-2">{actions}</div> : null}
    </header>
  );
}
