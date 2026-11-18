import type { ButtonHTMLAttributes } from "react";

type Variant = "primary" | "secondary" | "danger" | "quiet";

const styles: Record<Variant, string> = {
  primary: "bg-signal text-white hover:bg-[#183f94] disabled:bg-rule",
  secondary: "bg-surface text-ink border border-rule hover:border-ink-soft disabled:text-state-retired",
  danger: "bg-surface text-state-rejected border border-rule hover:border-state-rejected",
  quiet: "text-signal hover:underline px-0",
};

type Props = ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; busy?: boolean };

export function Button({ variant = "secondary", busy, className = "", children, disabled, ...rest }: Props) {
  return (
    <button
      type="button"
      {...rest}
      disabled={disabled || busy}
      aria-busy={busy || undefined}
      className={`inline-flex items-center gap-2 rounded-[var(--radius-control)] px-3.5 py-2 text-sm font-semibold
        disabled:cursor-not-allowed ${styles[variant]} ${className}`}
    >
      {children}
    </button>
  );
}
