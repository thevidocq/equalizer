import { useId } from "react";

/** The three blades of the Equalizer mark, blue to green. */
const blades = [
  "M10 18C12 8 18 3 28 3L60 3C56 11 50 14 41 14L22 14C16 14 12 15.5 10 18Z",
  "M6 34C8 24 14 19.5 24 19.5L54 19.5C50 27 44 30.5 35 30.5L18 30.5C12 30.5 8 32 6 34Z",
  "M2 50C4 40 10 36 20 36L48 36C44 43.5 38 47 29 47L14 47C8 47 4 48 2 50Z",
];

export function LogoMark({ size = 28 }: { size?: number }) {
  const id = useId();
  return (
    <svg width={size} height={(size * 53) / 64} viewBox="0 0 64 53" aria-hidden="true" className="shrink-0">
      <defs>
        <linearGradient id={id} x1="8" y1="0" x2="40" y2="53" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#2F8CF0" />
          <stop offset="1" stopColor="#5BDC84" />
        </linearGradient>
      </defs>
      <g fill={`url(#${id})`}>{blades.map((d) => <path key={d} d={d} />)}</g>
    </svg>
  );
}

type Props = { size?: "sm" | "lg"; inverted?: boolean; tagline?: boolean };

/** Mark, name and tagline. */
export function Logo({ size = "sm", inverted = false, tagline = true }: Props) {
  const lg = size === "lg";
  return (
    <div className={`flex items-center ${lg ? "gap-4" : "gap-2.5"}`}>
      <LogoMark size={lg ? 76 : 34} />
      <div className="flex flex-col">
        <span className={`${lg ? "text-[40px]" : "text-[21px]"} font-semibold leading-none tracking-[-0.01em] ${inverted ? "text-paper" : "text-ink"}`}>
          Equalizer
        </span>
        {tagline ? <Tagline lg={lg} inverted={inverted} /> : null}
      </div>
    </div>
  );
}

/**
 * The tagline spans exactly the width of the name, never more: the box
 * takes no width of its own (w-0 min-w-full) and the letters are spaced to
 * fill it (textLength).
 */
function Tagline({ lg, inverted }: { lg: boolean; inverted: boolean }) {
  const size = lg ? 10.5 : 6.4;
  const height = Math.ceil(size * 1.3);
  return (
    <div className={`w-0 min-w-full ${lg ? "mt-2.5" : "mt-1.5"}`}>
      <svg width="100%" height={height} aria-label="Protect legacy web apps" role="img" className="block overflow-visible">
        <text
          x="0" y={size} textLength="100%" lengthAdjust="spacing" fontSize={size} fontWeight={500}
          fill="currentColor" className={inverted ? "text-paper/70" : "text-ink-soft"}
        >
          PROTECT LEGACY WEB APPS
        </text>
      </svg>
    </div>
  );
}
