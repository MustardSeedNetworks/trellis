/**
 * jsdom implements no `window.matchMedia`, so any code that asks the browser
 * about the OS colour scheme throws under vitest. A stub that always answered
 * "no" would be worse than none: `useTheme` exists precisely to react to that
 * answer and to changes in it, and a frozen stub makes the tests that prove it
 * pass for the wrong reason.
 *
 * This one is controllable per query. `setSystemColorScheme` flips an answer
 * and notifies the listeners registered for that query, which is what a real
 * browser does.
 */
type Listener = (event: MediaQueryListEvent) => void;

const DARK_QUERY = '(prefers-color-scheme: dark)';

const matching = new Map<string, boolean>();
const listeners = new Map<string, Set<Listener>>();

function listenersFor(query: string): Set<Listener> {
  const existing = listeners.get(query);
  if (existing) {
    return existing;
  }
  const created = new Set<Listener>();
  listeners.set(query, created);
  return created;
}

/** Sets what the stubbed browser answers for one query, notifying its listeners. */
function setMatches(query: string, matches: boolean): void {
  matching.set(query, matches);
  for (const listener of listenersFor(query)) {
    listener({ matches, media: query } as MediaQueryListEvent);
  }
}

/** Sets what the stubbed OS reports, notifying live listeners as a real one does. */
export function setSystemColorScheme(scheme: 'light' | 'dark'): void {
  setMatches(DARK_QUERY, scheme === 'dark');
}

/** Installs the stub on `window`. Called once from test-setup. */
export function installMatchMediaStub(): void {
  matching.clear();
  listeners.clear();
  window.matchMedia = ((query: string) => {
    return {
      get matches() {
        return matching.get(query) ?? false;
      },
      media: query,
      onchange: null,
      addEventListener: (_type: string, listener: Listener) => {
        listenersFor(query).add(listener);
      },
      removeEventListener: (_type: string, listener: Listener) => {
        listenersFor(query).delete(listener);
      },
      // Deprecated pair, still on the interface; nothing in this repo calls them.
      addListener: (listener: Listener) => {
        listenersFor(query).add(listener);
      },
      removeListener: (listener: Listener) => {
        listenersFor(query).delete(listener);
      },
      dispatchEvent: () => false,
    } as MediaQueryList;
  }) as typeof window.matchMedia;
}
