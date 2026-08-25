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
import { renderHook } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { useNavGroups } from '@/navGroups';
import { usePages } from '@/pageRegistry';

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
