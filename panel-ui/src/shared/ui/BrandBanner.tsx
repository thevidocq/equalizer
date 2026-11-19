import type { ReactNode } from "react";

/** Dark band with the brand's light waves, behind the large logo. */
export function BrandBanner({ children }: { children: ReactNode }) {
  return (
    <div className="relative overflow-hidden rounded-[var(--radius-panel)] bg-[#0E1A2B] px-6 pb-14 pt-8">
      <svg className="pointer-events-none absolute inset-x-0 bottom-0 h-12 w-full" viewBox="0 0 400 80" preserveAspectRatio="none" aria-hidden="true">
        <defs>
          <linearGradient id="eq-wave" x1="0" x2="1">
            <stop offset="0" stopColor="#2F8CF0" stopOpacity="0.1" />
            <stop offset="0.5" stopColor="#2F8CF0" stopOpacity="0.45" />
            <stop offset="1" stopColor="#5BDC84" stopOpacity="0.7" />
          </linearGradient>
        </defs>
        {[0, 6, 12, 18].map((o) => (
          <path key={o} d={`M0 ${40 + o} C100 ${10 + o} 180 ${75 - o / 2} 280 ${45 + o / 2} S380 ${20 + o} 400 ${30 + o}`} fill="none" stroke="url(#eq-wave)" strokeWidth="0.8" />
        ))}
      </svg>
      <div className="relative">{children}</div>
    </div>
  );
}
