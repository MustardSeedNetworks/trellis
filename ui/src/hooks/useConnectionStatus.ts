import { useEffect, useState } from 'react';

export type ConnectionState = 'checking' | 'connected' | 'disconnected';

export const CHECK_INTERVAL_MS = 15_000;
export const CHECK_TIMEOUT_MS = 5_000;

/**
 * Whether the daemon answers, asked again every fifteen seconds.
 *
 * The version endpoint is the probe because it is unauthenticated and does no
 * work. A reply of any kind that is a success counts as connected; a refused
 * connection, a timeout or a server error is disconnected, and the next tick
 * asks again, so a daemon that comes back clears the state without a reload.
 */
export function useConnectionStatus(): ConnectionState {
  const [status, setStatus] = useState<ConnectionState>('checking');

  useEffect(() => {
    let live = true;
    let controller = new AbortController();

    const check = async () => {
      // A fresh controller per check: one that timed out stays aborted.
      controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), CHECK_TIMEOUT_MS);
      try {
        const response = await fetch('/__version', { signal: controller.signal });
        if (live) {
          setStatus(response.ok ? 'connected' : 'disconnected');
        }
      } catch {
        if (live) {
          setStatus('disconnected');
        }
      } finally {
        clearTimeout(timeout);
      }
    };

    void check();
    const interval = setInterval(check, CHECK_INTERVAL_MS);
    return () => {
      live = false;
      clearInterval(interval);
      controller.abort();
    };
  }, []);

  return status;
}
