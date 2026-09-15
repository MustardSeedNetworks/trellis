/**
 * jsdom implements no `window.matchMedia`, so any code that asks the browser
 * for the OS colour scheme throws under vitest. A stub that always answers
 * "light" would be worse than none: `useTheme`'s whole job is to react to that
 * answer and to changes in it, and a frozen stub makes the test that proves it
 * pass for the wrong reason.
 *
 * This one is controllable. `setSystemColorScheme` flips the answer and
 * notifies every listener, which is what a real OS theme change does.
 */
type Listener = (event: MediaQueryListEvent) => void;

const DARK_QUERY = '(prefers-color-scheme: dark)';

let prefersDark = false;
const listeners = new Set<Listener>();

/** Sets what the stubbed OS reports, notifying live listeners as a real one does. */
export function setSystemColorScheme(scheme: 'light' | 'dark'): void {
  prefersDark = scheme === 'dark';
  for (const listener of listeners) {
    listener({ matches: prefersDark, media: DARK_QUERY } as MediaQueryListEvent);
  }
}

/** Installs the stub on `window`. Called once from test-setup. */
export function installMatchMediaStub(): void {
  window.matchMedia = ((query: string) => {
    const matches = query === DARK_QUERY ? prefersDark : false;
    return {
      matches,
      media: query,
      onchange: null,
      addEventListener: (_type: string, listener: Listener) => {
        listeners.add(listener);
      },
      removeEventListener: (_type: string, listener: Listener) => {
        listeners.delete(listener);
      },
      // Deprecated pair, still on the interface; nothing in this repo calls them.
      addListener: (listener: Listener) => {
        listeners.add(listener);
      },
      removeListener: (listener: Listener) => {
        listeners.delete(listener);
      },
      dispatchEvent: () => false,
    } as MediaQueryList;
  }) as typeof window.matchMedia;
}
