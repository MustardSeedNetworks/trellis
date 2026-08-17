/**
 * Guards the rail <-> registry relationship, which is one-directional
 * here on purpose: every routed page must appear in the rail, but the
 * rail deliberately lists sections whose pages are not built yet, so
 * the reverse assertion the siblings make would fail by design.
 */
import { describe, expect, it } from 'vitest';
import { navGroups } from '@/navGroups';
import { pages } from '@/pageRegistry';

describe('pageRegistry <-> navGroups', () => {
  const navPaths = new Set(navGroups.flatMap((group) => group.items.map((item) => item.path)));

  it('exposes every routable page in the rail', () => {
    const missing = pages.map((page) => page.path).filter((path) => !navPaths.has(path));
    expect(missing, `pages missing from navGroups: ${missing.join(', ')}`).toEqual([]);
  });

  it('titles each page the same as its rail entry', () => {
    const railLabel = new Map(
      navGroups.flatMap((group) => group.items.map((item) => [item.path, item.label] as const)),
    );
    for (const page of pages) {
      expect(page.title, `${page.path} title`).toBe(railLabel.get(page.path));
    }
  });
});
