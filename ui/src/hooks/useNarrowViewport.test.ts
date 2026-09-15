import { act, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { useNarrowViewport } from '@/hooks/useNarrowViewport';
import { setViewportNarrow } from '@/test/matchMedia';

afterEach(() => setViewportNarrow(false));

describe('useNarrowViewport', () => {
  it('reports a wide viewport as not narrow', () => {
    const { result } = renderHook(() => useNarrowViewport());

    expect(result.current).toBe(false);
  });

  it('reports a phone-width viewport as narrow on first render', () => {
    setViewportNarrow(true);

    const { result } = renderHook(() => useNarrowViewport());

    expect(result.current).toBe(true);
  });

  it('follows a viewport that changes while the shell is mounted', () => {
    const { result } = renderHook(() => useNarrowViewport());
    expect(result.current).toBe(false);

    act(() => setViewportNarrow(true));
    expect(result.current).toBe(true);

    act(() => setViewportNarrow(false));
    expect(result.current).toBe(false);
  });
});
