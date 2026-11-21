import type { ReactNode } from 'react';
import { createContext, use, useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { DaemonUnreachableError, fetchSession, logout, onSessionLost } from '@/lib/auth';
import { LoginPage } from '@/pages/LoginPage';
import { DaemonUnreachable } from '@/ui/DaemonUnreachable';

interface AuthState {
  /** True only on a daemon that asked us to log in — a loopback one never does. */
  required: boolean;
  signOut: () => void;
}

const AuthContext = createContext<AuthState>({ required: false, signOut: () => {} });

/** useAuth lets the shell offer a sign-out only where there is a session to end. */
export function useAuth(): AuthState {
  return use(AuthContext);
}

/** How long to wait before asking an unreachable daemon again. */
export const PROBE_RETRY_MS = 5_000;

/**
 * AuthGate decides whether the operator sees the app or a login form.
 *
 * A daemon bound to loopback registers no auth routes, the probe 404s, and this
 * renders children immediately — the desktop app is unchanged by this feature.
 * A daemon serving other devices answers the probe and the shell waits behind
 * the form. A session the daemon later refuses to renew sends the operator
 * back to the form, saying why.
 *
 * A daemon that cannot be reached is neither of those. The form would invite an
 * operator to type a password at nothing, and the app would render a shell of
 * pages that all fail, so this says the daemon is unreachable and asks again
 * until it answers.
 */
export function AuthGate({ children }: { children: ReactNode }) {
  const { t } = useTranslation('pages');
  const [authenticated, setAuthenticated] = useState<boolean | null>(null);
  const [required, setRequired] = useState(false);
  const [unreachable, setUnreachable] = useState(false);
  // Set only when the daemon refused to renew a session that had been live. A
  // deliberate sign-out is not an expiry, and must not read as one.
  const [expired, setExpired] = useState(false);

  const signOut = useCallback(() => {
    void logout().finally(() => {
      setExpired(false);
      setAuthenticated(false);
    });
  }, []);
  const state = useMemo<AuthState>(() => ({ required, signOut }), [required, signOut]);

  useEffect(
    () =>
      onSessionLost(() => {
        setExpired(true);
        setAuthenticated(false);
      }),
    [],
  );

  useEffect(() => {
    let live = true;
    let retry: ReturnType<typeof setTimeout> | undefined;

    const probe = () => {
      fetchSession()
        .then((session) => {
          if (live) {
            setUnreachable(false);
            setAuthenticated(session.authenticated);
            setRequired(!session.authDisabled);
          }
        })
        .catch((error: unknown) => {
          if (!live) {
            return;
          }
          if (error instanceof DaemonUnreachableError) {
            setUnreachable(true);
            retry = setTimeout(probe, PROBE_RETRY_MS);
            return;
          }
          // The daemon answered with something that is not a session — a dev
          // server with no daemon behind it serves HTML here. That is not an
          // authentication answer either; the app renders and its own error
          // states say what is wrong.
          setUnreachable(false);
          setAuthenticated(true);
        });
    };
    probe();

    return () => {
      live = false;
      clearTimeout(retry);
    };
  }, []);

  if (unreachable) {
    return <DaemonUnreachable fullScreen />;
  }
  if (authenticated === null) {
    return (
      <div
        className="flex h-screen items-center justify-center bg-surface-base text-sm text-text-secondary"
        data-testid="auth-checking"
      >
        {t('login.checking')}
      </div>
    );
  }
  if (!authenticated) {
    return (
      <LoginPage
        sessionExpired={expired}
        onAuthenticated={() => {
          setExpired(false);
          setAuthenticated(true);
        }}
      />
    );
  }
  return <AuthContext value={state}>{children}</AuthContext>;
}
