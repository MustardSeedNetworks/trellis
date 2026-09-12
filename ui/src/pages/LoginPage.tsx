import { type FormEvent, useId, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { LoginError, type LoginFailure, login } from '@/lib/auth';

type Failure = LoginFailure | 'empty';

/**
 * Each failure's message comes from its own literal t() call.
 *
 * A template key — t(`login.errors.${failure}`) — reads better and is invisible
 * to extraction, which then reports the four strings as unused and deletes
 * them. A lookup table of key strings is invisible the same way: the gate scans
 * for t() calls, not for constants that happen to hold key names.
 */
function useErrorMessage(): (failure: Failure) => string {
  const { t } = useTranslation('pages');

  return (failure) => {
    switch (failure) {
      case 'invalid':
        return t('login.errors.invalid');
      case 'rateLimited':
        return t('login.errors.rateLimited');
      case 'unavailable':
        return t('login.errors.unavailable');
      case 'empty':
        return t('login.errors.empty');
    }
  };
}

/**
 * The login form a protected daemon shows instead of the shell.
 *
 * It is deliberately the whole viewport rather than a modal over the app: there
 * is nothing behind it to look at, and a survey tool that appeared to be
 * loading data it cannot read would be worse than one that says what it wants.
 */
export function LoginPage({ onAuthenticated }: { onAuthenticated: (username: string) => void }) {
  const { t } = useTranslation('pages');
  const errorMessage = useErrorMessage();
  const usernameId = useId();
  const passwordId = useId();

  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [failure, setFailure] = useState<Failure | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    if (!username || !password) {
      setFailure('empty');
      return;
    }
    setSubmitting(true);
    setFailure(null);
    try {
      const session = await login(username, password);
      onAuthenticated(session.username ?? username);
    } catch (error) {
      setFailure(error instanceof LoginError ? error.reason : 'unavailable');
      // The password field is cleared on failure; the username is kept, since
      // retyping the half that was probably right is friction, not security.
      setPassword('');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="flex h-screen items-center justify-center bg-surface-base px-4 text-text-primary">
      <form
        onSubmit={handleSubmit}
        className="panel w-full max-w-sm p-6"
        data-testid="login-form"
        aria-labelledby={`${usernameId}-heading`}
      >
        <h1 id={`${usernameId}-heading`} className="text-lg font-semibold">
          {t('login.title')}
        </h1>
        <p className="mt-1 text-sm text-text-secondary">{t('login.subtitle')}</p>

        <label htmlFor={usernameId} className="mt-6 block text-sm text-text-secondary">
          {t('login.username')}
        </label>
        <input
          id={usernameId}
          data-testid="login-username"
          className="mt-1 w-full rounded border border-hairline bg-surface-raised px-3 py-2 text-sm"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          autoComplete="username"
        />

        <label htmlFor={passwordId} className="mt-4 block text-sm text-text-secondary">
          {t('login.password')}
        </label>
        <input
          id={passwordId}
          data-testid="login-password"
          type="password"
          className="mt-1 w-full rounded border border-hairline bg-surface-raised px-3 py-2 text-sm"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="current-password"
        />

        {failure && (
          <p className="mt-4 text-sm text-status-error" role="alert" data-testid="login-error">
            {errorMessage(failure)}
          </p>
        )}

        <button
          type="submit"
          disabled={submitting}
          data-testid="login-submit"
          className="mt-6 w-full rounded border border-hairline px-3 py-2 text-sm hover:bg-surface-raised disabled:opacity-60"
        >
          {submitting ? t('login.submitting') : t('login.submit')}
        </button>
      </form>
    </div>
  );
}
