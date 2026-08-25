import { describe, expect, it } from 'vitest';
import { resolveApiBaseUrl } from './client';

/**
 * The UI shipped for months pointed at a compile-time loopback address while
 * being served from whatever port trellisd bound, so every RPC was cross-origin
 * and the browser blocked it. These assert the resolution itself, since that is
 * the whole of the decision.
 */
describe('resolveApiBaseUrl', () => {
  it('calls the origin it was served from when nothing is configured', () => {
    expect(resolveApiBaseUrl(undefined, 'http://192.168.1.10:18099')).toBe(
      'http://192.168.1.10:18099',
    );
  });

  it('never falls back to a fixed loopback address', () => {
    // Over the network that address is the viewer's own machine, not the server.
    expect(resolveApiBaseUrl(undefined, 'https://trellis.msn.lab')).not.toContain('127.0.0.1');
  });

  it('honours an explicit override, for a dev server on a separate origin', () => {
    expect(resolveApiBaseUrl('http://127.0.0.1:8446', 'http://localhost:5173')).toBe(
      'http://127.0.0.1:8446',
    );
  });

  it('treats a blank override as unset, so a stray empty env var is not a broken base URL', () => {
    expect(resolveApiBaseUrl('   ', 'http://127.0.0.1:18099')).toBe('http://127.0.0.1:18099');
  });
});
