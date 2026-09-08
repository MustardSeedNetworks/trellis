import { Code, ConnectError } from '@connectrpc/connect';
import { QueryClient } from '@tanstack/react-query';

/**
 * Codes the daemon has already decided. Asking again returns the same answer,
 * so a retry buys nothing and costs the operator the loading branch for the
 * whole of the backoff (#389).
 */
const answered = new Set<Code>([
  Code.InvalidArgument,
  Code.NotFound,
  Code.PermissionDenied,
  Code.Unauthenticated,
  Code.AlreadyExists,
]);

/**
 * retryQuery keeps TanStack Query's default three attempts for anything that
 * might answer differently next time -- a dropped socket, an unavailable or
 * internal daemon -- and stops immediately on a client-class Connect code.
 *
 * ConnectError.from maps a non-Connect rejection (a transport TypeError) to
 * Code.Unknown, which is deliberately not in the set above: that is the case
 * the retry policy exists for.
 */
export function retryQuery(failureCount: number, error: unknown): boolean {
  if (answered.has(ConnectError.from(error).code)) {
    return false;
  }
  return failureCount < 3;
}

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: retryQuery },
    },
  });
}
