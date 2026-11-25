import { type FormEvent, useId, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { LoginError, type LoginFailure, login } from '@/lib/auth';
import { Alert } from '@/shared/Alert';
import { Button } from '@/shared/Button';
import { FormField } from '@/shared/FormField';
import { Input } from '@/shared/Input';
import { ProductMark } from '@/ui/ProductMark';

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
export function LoginPage({
  onAuthenticated,
  sessionExpired = false,
}: {
  onAuthenticated: (username: string) => void;
  /** The daemon refused to renew a session that was live: say so before asking again. */
  sessionExpired?: boolean;
}) {
  const { t } = useTranslation('pages');
  const errorMessage = useErrorMessage();
  const headingId = useId();
  const errorId = useId();

  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [failure, setFailure] = useState<Failure | null>(null);
  const [submitting, setSubmitting] = useState(false);
  // Only the two failures about what was typed mark the fields invalid; a
  // daemon that is down or rate-limiting has nothing wrong with the input.
  const invalid = failure === 'empty' || failure === 'invalid';

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
        aria-labelledby={headingId}
      >
        <div className="flex items-center gap-3">
          <ProductMark />
          <div className="min-w-0">
            {/* allow-hardcoded: product name, not copy. */}
            <h1 className="text-xl font-extrabold tracking-tight">Trellis</h1>
            <p className="text-sm text-text-secondary">{t('login.tagline')}</p>
          </div>
        </div>

        <h2 id={headingId} className="mt-6 text-lg font-semibold">
          {t('login.title')}
        </h2>
        <p className="mt-1 text-sm text-text-secondary">{t('login.subtitle')}</p>

        <div className="mt-4 flex flex-col gap-4">
          <FormField label={t('login.username')}>
            {(control) => (
              <Input
                {...control}
                data-testid="login-username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoComplete="username"
                aria-invalid={invalid}
                aria-describedby={failure ? errorId : undefined}
              />
            )}
          </FormField>

          <FormField label={t('login.password')}>
            {(control) => (
              <Input
                {...control}
                data-testid="login-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
                aria-invalid={invalid}
                aria-describedby={failure ? errorId : undefined}
              />
            )}
          </FormField>
        </div>

        {sessionExpired && !failure ? (
          <div className="mt-4" data-testid="login-session-expired">
            <Alert tone="warning">{t('login.sessionExpired')}</Alert>
          </div>
        ) : null}

        {failure && (
          <div id={errorId} className="mt-4" data-testid="login-error">
            <Alert tone="error">{errorMessage(failure)}</Alert>
          </div>
        )}

        <Button
          type="submit"
          variant="primary"
          loading={submitting}
          data-testid="login-submit"
          className="mt-6 w-full"
        >
          {submitting ? t('login.submitting') : t('login.submit')}
        </Button>
      </form>
    </div>
  );
}
