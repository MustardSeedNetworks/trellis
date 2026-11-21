import { AlertCircle, RefreshCw } from 'lucide-react';
import { Component, type ErrorInfo, type ReactNode } from 'react';
// A boundary has to be a class, so it cannot call useTranslation. <Trans> is a
// component and subscribes to language changes, so the fallback still follows
// the locale.
import { Trans } from 'react-i18next';
import { iconSizes } from '../constants/sizes';

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
}

/**
 * Shared by both boundaries: what happens to the tree after a render throws.
 * The error itself goes to the console with its component stack, which is the
 * one place an operator reporting a defect can copy it from.
 */
abstract class Boundary extends Component<Props, State> {
  state: State = { hasError: false };

  static getDerivedStateFromError(): State {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error('render error', error, info.componentStack);
  }

  protected retry = (): void => {
    this.setState({ hasError: false });
  };

  protected reload = (): void => {
    window.location.reload();
  };
}

/**
 * The whole app. It sits outside the router and the providers, so a render
 * error anywhere below — including in them — lands here instead of leaving a
 * white screen. "Home" is a plain link for the same reason: there is no router
 * above this to navigate with, and a full load is what a recovery wants anyway.
 */
export class ErrorBoundary extends Boundary {
  render(): ReactNode {
    if (!this.state.hasError) {
      return this.props.children;
    }
    return (
      <div
        role="alert"
        data-testid="error-boundary-fallback"
        className="flex h-screen items-center justify-center bg-surface-base px-4 text-text-primary"
      >
        <div className="panel w-full max-w-md p-6 text-center">
          <AlertCircle className={`${iconSizes['3xl']} mx-auto text-status-error`} aria-hidden />
          <h1 className="mt-4 text-lg font-semibold">
            <Trans i18nKey="errorBoundary.title" ns="common" />
          </h1>
          <p className="mt-2 text-sm text-text-secondary">
            <Trans i18nKey="errorBoundary.description" ns="common" />
          </p>
          <div className="mt-6 flex justify-center gap-3">
            <button
              type="button"
              onClick={this.reload}
              data-testid="error-boundary-reload"
              className="inline-flex min-h-11 items-center gap-2 rounded bg-brand-primary px-4 py-2 text-sm font-medium text-on-brand hover:bg-brand-accent"
            >
              <RefreshCw className={iconSizes.md} aria-hidden />
              <Trans i18nKey="errorBoundary.reload" ns="common" />
            </button>
            <a
              href="/"
              data-testid="error-boundary-home"
              className="inline-flex min-h-11 items-center rounded border border-hairline px-4 py-2 text-sm text-text-primary hover:bg-surface-raised"
            >
              <Trans i18nKey="errorBoundary.home" ns="common" />
            </a>
          </div>
        </div>
      </div>
    );
  }
}

/**
 * One page. The rail and header stay up and the operator can navigate away;
 * the caller keys it by path so the error does not follow them to the next
 * page.
 */
export class PageErrorBoundary extends Boundary {
  render(): ReactNode {
    if (!this.state.hasError) {
      return this.props.children;
    }
    return (
      <div className="p-6">
        <div
          role="alert"
          data-testid="page-error-boundary-fallback"
          className="panel flex items-start gap-3 border-status-error p-4"
        >
          <AlertCircle className={`${iconSizes.xl} shrink-0 text-status-error`} aria-hidden />
          <div className="flex-1">
            <h2 className="text-sm font-medium text-status-error-strong">
              <Trans i18nKey="errorBoundary.pageTitle" ns="common" />
            </h2>
            <p className="mt-1 text-sm text-text-secondary">
              <Trans i18nKey="errorBoundary.pageDescription" ns="common" />
            </p>
            <div className="mt-3 flex gap-4">
              <button
                type="button"
                onClick={this.retry}
                data-testid="page-error-boundary-retry"
                className="text-sm font-medium text-text-primary hover:underline"
              >
                <Trans i18nKey="errorBoundary.retry" ns="common" />
              </button>
              <button
                type="button"
                onClick={this.reload}
                className="text-sm font-medium text-text-primary hover:underline"
              >
                <Trans i18nKey="errorBoundary.reload" ns="common" />
              </button>
            </div>
          </div>
        </div>
      </div>
    );
  }
}
