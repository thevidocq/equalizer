import type { ReactNode } from "react";
import { BrandBanner, Logo } from "@/shared/ui";

/** The narrow column of the login steps. */
export function AuthFrame({ title, children, wide = false }: { title: string; children: ReactNode; wide?: boolean }) {
  return (
    <main className="grid min-h-screen place-items-center px-4 py-10">
      <div className={`w-full ${wide ? "max-w-md" : "max-w-sm"}`}>
        <BrandBanner><Logo size="lg" inverted /></BrandBanner>
        <div className="mt-6 flex flex-col gap-4 rounded-[var(--radius-panel)] border border-rule bg-surface p-6">
          <h1 className="text-lg font-semibold">{title}</h1>
          {children}
        </div>
      </div>
    </main>
  );
}
