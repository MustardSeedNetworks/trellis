import type { ReactNode } from 'react';
import { createContext, use, useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { fetchSession, logout } from '@/lib/auth';
import { LoginPage } from '@/pages/LoginPage';

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

/**
 * AuthGate decides whether the operator sees the app or a login form.
 *
 * A daemon bound to loopback registers no auth routes, the probe 404s, and this
 * renders children immediately — the desktop app is unchanged by this feature.
 * A daemon serving other devices answers the probe and the shell waits behind
 * the form.
 */
export function AuthGate({ children }: { children: ReactNode }) {
  const { t } = useTranslation('pages');
  const [authenticated, setAuthenticated] = useState<boolean | null>(null);
  const [required, setRequired] = useState(false);

  const signOut = useCallback(() => {
    void logout().finally(() => setAuthenticated(false));
  }, []);
  const state = useMemo<AuthState>(() => ({ required, signOut }), [required, signOut]);

  useEffect(() => {
    let live = true;
    fetchSession()
      .then((session) => {
        if (live) {
          setAuthenticated(session.authenticated);
          setRequired(!session.authDisabled);
        }
      })
      .catch(() => {
        // An unreachable daemon is not an authentication answer. Showing the
        // form would invite an operator to type a password at nothing; the app
        // renders and its own error states say what is wrong.
        if (live) {
          setAuthenticated(true);
        }
      });
    return () => {
      live = false;
    };
  }, []);

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
    return <LoginPage onAuthenticated={() => setAuthenticated(true)} />;
  }
  return <AuthContext value={state}>{children}</AuthContext>;
}
