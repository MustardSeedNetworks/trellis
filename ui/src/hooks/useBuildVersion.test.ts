import { renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useBuildVersion } from './useBuildVersion';

afterEach(() => vi.unstubAllGlobals());

describe('useBuildVersion', () => {
  it('reports the version the running daemon serves from /__version', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          version: '0.12.44',
          commit: 'abc1234',
          buildTime: 'now',
          uiBuildHash: 'h',
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    );
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useBuildVersion());

    await waitFor(() => expect(result.current.version).toBe('0.12.44'));
    expect(fetchMock).toHaveBeenCalledWith('/__version', expect.anything());
    expect(result.current.uiBuildHash).toBe('h');
  });

  it('falls back to dev when there is no daemon to ask', async () => {
    const fetchMock = vi.fn().mockRejectedValue(new TypeError('network'));
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useBuildVersion());

    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    expect(result.current.version).toBe('dev');
  });

  it('ignores a reply that is not build metadata', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('<!doctype html>', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useBuildVersion());

    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    expect(result.current.version).toBe('dev');
  });
});
