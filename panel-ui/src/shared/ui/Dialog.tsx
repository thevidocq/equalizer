import * as RD from "@radix-ui/react-dialog";
import type { ReactNode } from "react";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string;
  children: ReactNode;
};

export function Dialog({ open, onOpenChange, title, description, children }: Props) {
  return (
    <RD.Root open={open} onOpenChange={onOpenChange}>
      <RD.Portal>
        <RD.Overlay className="fixed inset-0 z-50 bg-ink/30" />
        <RD.Content
          className="fixed left-1/2 top-[12vh] z-50 w-[min(34rem,calc(100vw-2rem))] -translate-x-1/2
            rounded-[var(--radius-panel)] border border-rule bg-surface p-6"
        >
          <RD.Title className="text-lg font-semibold">{title}</RD.Title>
          {description ? <RD.Description className="mt-1 text-sm text-ink-soft">{description}</RD.Description> : null}
          <div className="mt-5">{children}</div>
          <RD.Close className="absolute right-4 top-4 text-ink-soft hover:text-ink" aria-label="Close">
            <span aria-hidden="true">✕</span>
          </RD.Close>
        </RD.Content>
      </RD.Portal>
    </RD.Root>
  );
}
