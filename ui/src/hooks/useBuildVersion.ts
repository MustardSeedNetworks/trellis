import { useEffect, useState } from 'react';

/**
 * Build metadata served by the daemon's unauthenticated /__version endpoint.
 * The lowercase keys are the Universal Build Contract's; seed, stem and niac
 * serve the same shape.
 */
export interface BuildVersion {
  version: string;
  commit: string;
  buildTime: string;
  uiBuildHash: string;
}

const FALLBACK: BuildVersion = {
  version: 'dev',
  commit: 'unknown',
  buildTime: 'unknown',
  uiBuildHash: '',
};

function isBuildVersion(value: unknown): value is BuildVersion {
  if (typeof value !== 'object' || value === null) {
    return false;
  }
  const candidate = value as Record<string, unknown>;
  return (
    typeof candidate.version === 'string' &&
    typeof candidate.commit === 'string' &&
    typeof candidate.buildTime === 'string' &&
    typeof candidate.uiBuildHash === 'string'
  );
}

/**
 * Fetches the running daemon's build metadata once on mount, so the rail shows
 * the version actually serving this page rather than the one this bundle was
 * built beside. Falls back to a "dev" descriptor when the endpoint does not
 * answer — under the Vite dev server there is no daemon — so callers render the
 * version without a loading state.
 */
export function useBuildVersion(): BuildVersion {
  const [build, setBuild] = useState<BuildVersion>(FALLBACK);

  useEffect(() => {
    let live = true;
    fetch('/__version', { headers: { Accept: 'application/json' } })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`status ${response.status}`);
        }
        return response.json() as Promise<unknown>;
      })
      .then((body) => {
        if (live && isBuildVersion(body)) {
          setBuild(body);
        }
      })
      .catch(() => {
        // FALLBACK is already in state.
      });
    return () => {
      live = false;
    };
  }, []);

  return build;
}
