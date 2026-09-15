/**
 * Is the shell being drawn on a phone-width screen?
 *
 * Trellis is walked around a floor, so the phone case is real work rather than
 * a courtesy: the rail was fixed at 252px with no responsive behaviour, which
 * left about 138px of a 390px screen for the page and clipped the Create
 * survey form mid-word (trellis#473).
 *
 * A media query rather than a resize listener: the browser already knows the
 * answer, and `matchMedia` fires once when the answer changes instead of on
 * every pixel of a drag. The breakpoint is Tailwind's `md` (768px), which is
 * the boundary the rest of this UI's utilities already use, so the rail and the
 * page content change shape together.
 */
import { useEffect, useState } from 'react';

/** Tailwind's `md` boundary, as the media query below it. */
const NARROW_QUERY = '(max-width: 767px)';

function matchesNarrow(): boolean {
  return window.matchMedia?.(NARROW_QUERY).matches ?? false;
}

export function useNarrowViewport(): boolean {
  const [narrow, setNarrow] = useState(matchesNarrow);

  useEffect(() => {
    const query = window.matchMedia?.(NARROW_QUERY);
    if (!query) {
      return;
    }
    // Read once here as well as in the initial state: a rotation or a resize
    // between the first render and this effect would otherwise be missed.
    setNarrow(query.matches);

    const handler = (event: MediaQueryListEvent): void => setNarrow(event.matches);
    query.addEventListener('change', handler);
    return (): void => query.removeEventListener('change', handler);
  }, []);

  return narrow;
}
