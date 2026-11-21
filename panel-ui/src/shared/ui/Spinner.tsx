/**
 * A small turning ring for work in progress. Still when the visitor asks
 * for reduced motion; the text next to it says what is going on.
 */
export function Spinner({ label }: { label?: string }) {
  return (
    <span role="status" className="inline-flex items-center gap-2">
      <span aria-hidden="true"
        className="inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-rule border-t-signal motion-reduce:animate-none" />
      {label ? <span>{label}</span> : <span className="sr-only">Working</span>}
    </span>
  );
}
