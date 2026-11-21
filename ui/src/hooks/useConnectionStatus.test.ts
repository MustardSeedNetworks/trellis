import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { CHECK_INTERVAL_MS, useConnectionStatus } from './useConnectionStatus';

beforeEach(() => vi.useFakeTimers());
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('useConnectionStatus', () => {
  it('starts checking, then reports a daemon that answers as connected', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}', { status: 200 })));
    const { result } = renderHook(() => useConnectionStatus());
    expect(result.current).toBe('checking');

    await act(() => vi.advanceTimersByTimeAsync(0));
    expect(result.current).toBe('connected');
  });

  it('reports a lost daemon and recovers on the next check without a reload', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response('{}', { status: 200 }))
      .mockRejectedValueOnce(new TypeError('network'))
      .mockResolvedValue(new Response('{}', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    const { result } = renderHook(() => useConnectionStatus());

    await act(() => vi.advanceTimersByTimeAsync(0));
    expect(result.current).toBe('connected');

    await act(() => vi.advanceTimersByTimeAsync(CHECK_INTERVAL_MS));
    expect(result.current).toBe('disconnected');

    await act(() => vi.advanceTimersByTimeAsync(CHECK_INTERVAL_MS));
    expect(result.current).toBe('connected');
  });

  it('treats a server error as disconnected', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('', { status: 502 })));
    const { result } = renderHook(() => useConnectionStatus());

    await act(() => vi.advanceTimersByTimeAsync(0));
    expect(result.current).toBe('disconnected');
  });

  it('stops asking once unmounted', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('{}', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    const { unmount } = renderHook(() => useConnectionStatus());
    await act(() => vi.advanceTimersByTimeAsync(0));

    unmount();
    await act(() => vi.advanceTimersByTimeAsync(CHECK_INTERVAL_MS * 3));
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
