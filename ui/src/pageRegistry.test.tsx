/**
 * Guards the rail <-> registry relationship, which is one-directional
 * here on purpose: every routed page must appear in the rail, but the
 * rail deliberately lists sections whose pages are not built yet, so
 * the reverse assertion the siblings make would fail by design.
 *
 * Both are hooks now that their labels are translated, so the assertions run
 * through renderHook. That makes this a copy check as well as a structural
 * one: a page whose title key is missing from the locale files renders as the
 * raw key and no longer matches its rail entry.
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, renderHook, screen, waitFor } from '@testing-library/react';
import { Suspense } from 'react';
import { MemoryRouter } from 'react-router';
import { describe, expect, it, vi } from 'vitest';
import { useNavGroups } from '@/navGroups';
import { usePages } from '@/pageRegistry';

vi.mock('@/lib/client', () => ({
  surveyClient: {
    listSurveys: vi.fn().mockResolvedValue({ surveys: [] }),
    generateReport: vi.fn(),
    createSurvey: vi.fn(),
    startSurvey: vi.fn(),
    capturePoint: vi.fn(),
    listSamples: vi.fn(),
    importAirMapper: vi.fn(),
    getHeatmap: vi.fn(),
    getCoverage: vi.fn().mockResolvedValue({
      coverageScore: 0,
      deadZoneCount: 0,
      recommendations: [],
    }),
  },
}));

describe('pageRegistry <-> navGroups', () => {
  it('exposes every routable page in the rail', () => {
    const { result: nav } = renderHook(() => useNavGroups());
    const { result: pages } = renderHook(() => usePages());

    const navPaths = new Set(nav.current.flatMap((group) => group.items.map((item) => item.path)));
    const missing = pages.current.map((page) => page.path).filter((path) => !navPaths.has(path));

    expect(missing, `pages missing from navGroups: ${missing.join(', ')}`).toEqual([]);
  });

  it('titles each page the same as its rail entry', () => {
    const { result: nav } = renderHook(() => useNavGroups());
    const { result: pages } = renderHook(() => usePages());

    const railLabel = new Map(
      nav.current.flatMap((group) => group.items.map((item) => [item.path, item.label] as const)),
    );
    for (const page of pages.current) {
      expect(page.title, `${page.path} title`).toBe(railLabel.get(page.path));
    }
  });

  it('resolves titles to copy rather than to raw keys', () => {
    const { result: pages } = renderHook(() => usePages());
    for (const page of pages.current) {
      expect(page.title, `${page.path} title`).not.toContain('.');
      expect(page.title, `${page.path} title`).not.toBe('');
    }
  });
});

/**
 * Each page.component is now React.lazy(), which resolves through a dynamic
 * import().then() mapping to the page's named export (see pageRegistry.tsx).
 * A typo in that mapping — the wrong export name, or an import path that
 * does not exist — throws only once React actually tries to render the lazy
 * component, which none of the per-page tests do (they import the named
 * export directly). This mounts each entry behind its own Suspense boundary
 * so a broken lazy mapping fails here instead of shipping silently.
 */
describe('pageRegistry lazy component mapping', () => {
  it("resolves every page's lazy component to real content", async () => {
    const { result: pages } = renderHook(() => usePages());
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    for (const page of pages.current) {
      const { unmount, container } = render(
        <QueryClientProvider client={client}>
          <MemoryRouter initialEntries={[page.path]}>
            <Suspense fallback="pending">
              <page.component />
            </Suspense>
          </MemoryRouter>
        </QueryClientProvider>,
      );

      await waitFor(
        () => {
          expect(screen.queryByText('pending')).not.toBeInTheDocument();
        },
        { timeout: 5000 },
      );
      expect(container, `${page.path} rendered nothing`).not.toBeEmptyDOMElement();

      unmount();
    }
    // Four sequential mounts, each waiting out its own lazy import and
    // data fetch; the default 5s test timeout is tight for that under load.
  }, 15000);
});
