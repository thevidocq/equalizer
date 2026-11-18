import { useId, type InputHTMLAttributes, type ReactNode, type TextareaHTMLAttributes } from "react";

type Common = { label: string; hint?: ReactNode; error?: string };

const control =
  "w-full rounded-[var(--radius-control)] border bg-surface px-3 py-2 text-[15px] text-ink placeholder:text-state-retired";

function Frame({ id, label, hint, error, children }: Common & { id: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-semibold">
        {label}
      </label>
      {children}
      {error ? (
        <p id={`${id}-msg`} className="text-sm text-state-rejected">
          {error}
        </p>
      ) : hint ? (
        <p id={`${id}-msg`} className="text-sm text-ink-soft">
          {hint}
        </p>
      ) : null}
    </div>
  );
}

export function TextField({ label, hint, error, ...rest }: Common & InputHTMLAttributes<HTMLInputElement>) {
  const id = useId();
  return (
    <Frame id={id} label={label} hint={hint} error={error}>
      <input
        id={id}
        {...rest}
        aria-invalid={error ? true : undefined}
        aria-describedby={error || hint ? `${id}-msg` : undefined}
        className={`${control} ${error ? "border-state-rejected" : "border-rule"}`}
      />
    </Frame>
  );
}

export function TextArea({ label, hint, error, className = "", ...rest }: Common & TextareaHTMLAttributes<HTMLTextAreaElement>) {
  const id = useId();
  return (
    <Frame id={id} label={label} hint={hint} error={error}>
      <textarea
        id={id}
        {...rest}
        aria-invalid={error ? true : undefined}
        aria-describedby={error || hint ? `${id}-msg` : undefined}
        className={`${control} ${error ? "border-state-rejected" : "border-rule"} ${className}`}
      />
    </Frame>
  );
}

export function SelectField({ label, hint, error, children, ...rest }: Common & InputHTMLAttributes<HTMLSelectElement> & { children: ReactNode }) {
  const id = useId();
  return (
    <Frame id={id} label={label} hint={hint} error={error}>
      <select id={id} {...(rest as object)} className={`${control} border-rule`}>
        {children}
      </select>
    </Frame>
  );
}
