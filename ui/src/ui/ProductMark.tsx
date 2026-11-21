/**
 * The Trellis product mark: the brand tile with its two-letter monogram.
 *
 * Two characters, never one: "T" for Stem and "R" for Trellis both read as
 * typos next to the other products. Shared by the rail lockup and the login
 * screen so the mark an operator signs in under is the one they work in.
 */
export function ProductMark({ className = '' }: { className?: string }) {
  return (
    <div
      aria-hidden="true"
      className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-[11px] bg-brand-primary ${className}`}
    >
      <span className="figure text-sm font-extrabold tracking-tight text-on-brand">TR</span>
    </div>
  );
}
