/**
 * Theme management.
 *
 * Trellis's dark palette has existed since the theme layer landed — the full
 * token set is in `theme/msn-shared.css` and `theme/product-trellis.css` under
 * `.dark` — but nothing ever added that class, so the palette was dead CSS and
 * the product rendered light on a dark desktop (trellis#472). This hook is the
 * missing half.
 *
 * Both stylesheets key off a plain `.dark` class rather than Tailwind's `dark:`
 * variant, and this repo uses no `dark:` utilities at all, so putting the class
 * on the document element is the whole mechanism.
 *
 * One store, many readers: the rail's quick toggle and the Settings control
 * both read this module's state through `useSyncExternalStore`, so a change in
 * one is on screen in the other in the same frame. A per-hook `useState` would
 * let the two drift apart and fight over the `dark` class.
 *
 * Persistence is per-browser `localStorage`, deliberately not the settings API:
 * Trellis is walked around a floor on whatever laptop is to hand, and a choice
 * made in a dim IDF should not follow the operator to a bright office monitor.
 *
 * The default is `system`, unlike seed / stem / niac, which all default to
 * `dark`. Their default predates the convention ("OS default, persisted
 * toggle") and it hides bugs: a test asserting the dark palette renders passes
 * in a light browser if the hook was going to pick dark either way.
 */
import { useCallback, useEffect, useSyncExternalStore } from 'react';

/** What the operator chose: an explicit colour, or "whatever the OS says". */
export type Theme = 'light' | 'dark' | 'system';

const STORAGE_KEY = 'trellis-theme';
const DARK_QUERY = '(prefers-color-scheme: dark)';

/** Held only when `localStorage` throws, so a blocked-storage browser keeps its choice for the page. */
let unpersisted: Theme | null = null;
const listeners = new Set<() => void>();

function notify(): void {
  for (const listener of listeners) {
    listener();
  }
}

/** What the OS reports right now. Light when it cannot be asked. */
function getSystemTheme(): 'light' | 'dark' {
  return window.matchMedia?.(DARK_QUERY).matches ? 'dark' : 'light';
}

/**
 * Reads the stored choice, defaulting to `system`.
 *
 * `localStorage` throws rather than returning null in a browser with site data
 * blocked, which would otherwise take the whole shell down on mount.
 */
function readTheme(): Theme {
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (stored === 'light' || stored === 'dark' || stored === 'system') {
      return stored;
    }
    return 'system';
  } catch {
    return unpersisted ?? 'system';
  }
}

function writeTheme(next: Theme): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, next);
  } catch {
    unpersisted = next;
  }
  notify();
}

function subscribeTheme(listener: () => void): () => void {
  listeners.add(listener);
  window.addEventListener('storage', listener);
  return () => {
    listeners.delete(listener);
    window.removeEventListener('storage', listener);
  };
}

// Tracked whatever the choice is, so that returning to `system` is instant and
// correct; `effectiveTheme` decides whether it is on screen.
function subscribeSystemTheme(listener: () => void): () => void {
  const query = window.matchMedia?.(DARK_QUERY);
  query?.addEventListener('change', listener);
  return () => query?.removeEventListener('change', listener);
}

/** Adds or removes the `dark` class the two stylesheets key off. */
function applyTheme(effective: 'light' | 'dark'): void {
  document.documentElement.classList.toggle('dark', effective === 'dark');
}

/**
 * Applies the stored (or OS) theme immediately, before React mounts.
 *
 * The hook alone is not enough: `AuthGate` renders the login screen above
 * `App`, so a hook called inside the shell would leave that screen light on a
 * dark desktop, and applying it in an effect would flash the wrong palette on
 * every load. main.tsx calls this once before `createRoot`.
 */
export function applyStoredTheme(): void {
  const stored = readTheme();
  applyTheme(stored === 'system' ? getSystemTheme() : stored);
}

export function useTheme(): {
  theme: Theme;
  effectiveTheme: 'light' | 'dark';
  setTheme: (next: Theme) => void;
  toggleTheme: () => void;
  isDark: boolean;
} {
  const theme = useSyncExternalStore(subscribeTheme, readTheme);
  const systemTheme = useSyncExternalStore(subscribeSystemTheme, getSystemTheme);
  const effectiveTheme = theme === 'system' ? systemTheme : theme;

  // Toggling is always an explicit choice, so it resolves `system` to whichever
  // colour is not on screen rather than cycling through a third state the quick
  // toggle cannot show. The Settings control is where `system` is chosen.
  const toggleTheme = useCallback(() => {
    writeTheme(effectiveTheme === 'dark' ? 'light' : 'dark');
  }, [effectiveTheme]);

  useEffect(() => {
    applyTheme(effectiveTheme);
  }, [effectiveTheme]);

  return {
    theme,
    effectiveTheme,
    setTheme: writeTheme,
    toggleTheme,
    isDark: effectiveTheme === 'dark',
  };
}
