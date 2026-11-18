/** The mark: three bars levelled to the same height, what the gateway does to every request. */
export function Wordmark({ inverted = false }: { inverted?: boolean }) {
  const ink = inverted ? "#F4F6F8" : "#1C2633";
  return (
    <div className="flex items-center gap-2.5">
      <svg width="22" height="22" viewBox="0 0 22 22" aria-hidden="true">
        <path d="M5 5v12M11 5v12M17 5v12" stroke={ink} strokeWidth="3" strokeLinecap="round" />
      </svg>
      <span className={`text-[17px] font-bold tracking-[-0.01em] ${inverted ? "text-paper" : "text-ink"}`}>Equalizer</span>
    </div>
  );
}
