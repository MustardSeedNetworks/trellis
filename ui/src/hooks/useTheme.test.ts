/**
 * The row this covers (UI-TRL-1, trellis#472) is not "a toggle is missing" —
 * the whole dark palette was already written and simply never applied, so the
 * audit's light and dark screenshots came back pixel-identical. These tests
 * assert the thing that was absent: that something puts the `dark` class on the
 * document element, and takes it off again.
 *
 * Trellis defaults to `system` where seed, stem and niac default to `dark`.
 * Their default predates the convention this row cites ("OS default, persisted
 * toggle") and it makes the acceptance vacuous: "dark tokens render" passes in a
 * light browser when the hook was going to choose dark regardless.
 */
import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { setSystemColorScheme } from '@/test/matchMedia';
import { useTheme } from './useTheme';

const STORAGE_KEY = 'trellis-theme';

beforeEach(() => {
  window.localStorage.clear();
  document.documentElement.classList.remove('dark');
  setSystemColorScheme('light');
});

afterEach(() => {
  document.documentElement.classList.remove('dark');
});

describe('useTheme', () => {
  it('follows a dark OS with no stored preference', () => {
    setSystemColorScheme('dark');

    const { result } = renderHook(() => useTheme());

    expect(result.current.theme).toBe('system');
    expect(result.current.isDark).toBe(true);
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });

  it('follows a light OS with no stored preference', () => {
    const { result } = renderHook(() => useTheme());

    expect(result.current.isDark).toBe(false);
    expect(document.documentElement.classList.contains('dark')).toBe(false);
  });

  it('lets a stored preference override the OS', () => {
    setSystemColorScheme('dark');
    window.localStorage.setItem(STORAGE_KEY, 'light');

    const { result } = renderHook(() => useTheme());

    expect(result.current.theme).toBe('light');
    expect(document.documentElement.classList.contains('dark')).toBe(false);
  });

  it('persists a toggle so the choice survives a reload', () => {
    const { result, unmount } = renderHook(() => useTheme());

    act(() => {
      result.current.toggleTheme();
    });

    expect(window.localStorage.getItem(STORAGE_KEY)).toBe('dark');
    expect(document.documentElement.classList.contains('dark')).toBe(true);

    unmount();
    document.documentElement.classList.remove('dark');
    const remounted = renderHook(() => useTheme());

    expect(remounted.result.current.isDark).toBe(true);
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });

  it('tracks a live OS change while on system', () => {
    const { result } = renderHook(() => useTheme());
    expect(result.current.isDark).toBe(false);

    act(() => {
      setSystemColorScheme('dark');
    });

    expect(result.current.isDark).toBe(true);
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });

  it('ignores a live OS change once the operator has chosen', () => {
    window.localStorage.setItem(STORAGE_KEY, 'light');
    const { result } = renderHook(() => useTheme());

    act(() => {
      setSystemColorScheme('dark');
    });

    expect(result.current.isDark).toBe(false);
    expect(document.documentElement.classList.contains('dark')).toBe(false);
  });
});
