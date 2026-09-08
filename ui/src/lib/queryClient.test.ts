import { Code, ConnectError } from '@connectrpc/connect';
import { describe, expect, it, vi } from 'vitest';
import { createQueryClient, retryQuery } from './queryClient';

describe('retryQuery', () => {
  // The daemon has already decided these; asking again spends a round trip and
  // a backoff to be told the same thing, while the page sits on its loading
  // branch (#389).
  it.each([
    ['invalid_argument', Code.InvalidArgument],
    ['not_found', Code.NotFound],
    ['permission_denied', Code.PermissionDenied],
    ['unauthenticated', Code.Unauthenticated],
    ['already_exists', Code.AlreadyExists],
  ])('does not retry %s', (_name, code) => {
    expect(retryQuery(0, new ConnectError('answered', code))).toBe(false);
  });

  it.each([
    ['unavailable', Code.Unavailable],
    ['internal', Code.Internal],
  ])('retries %s up to the default cap', (_name, code) => {
    const err = new ConnectError('transient', code);
    expect(retryQuery(0, err)).toBe(true);
    expect(retryQuery(2, err)).toBe(true);
    expect(retryQuery(3, err)).toBe(false);
  });

  // A dropped socket rejects with a TypeError, not a ConnectError.
  // ConnectError.from maps it to Code.Unknown, which must stay retryable --
  // this is the case the default policy exists for.
  it('retries a transport failure that is not a ConnectError', () => {
    expect(retryQuery(0, new TypeError('Failed to fetch'))).toBe(true);
  });
});

describe('createQueryClient', () => {
  it('stops after the first answer for a client-class code', async () => {
    const queryFn = vi.fn().mockRejectedValue(new ConnectError('no samples', Code.InvalidArgument));
    const client = createQueryClient();

    await expect(client.fetchQuery({ queryKey: ['heatmap', 'download'], queryFn })).rejects.toThrow(
      'no samples',
    );

    expect(queryFn).toHaveBeenCalledTimes(1);
  });

  it('still retries a transient failure', async () => {
    const queryFn = vi.fn().mockRejectedValue(new ConnectError('down', Code.Unavailable));
    const client = createQueryClient();

    await expect(
      // retryDelay is overridden because the default backoff is 1s/2s/4s and
      // this assertion is about the count, not the wait.
      client.fetchQuery({ queryKey: ['surveys'], queryFn, retryDelay: 0 }),
    ).rejects.toThrow('down');

    expect(queryFn).toHaveBeenCalledTimes(4);
  });
});
