import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ErrorBoundary, PageErrorBoundary } from './ErrorBoundary';

function Boom({ message = 'kaboom' }: { message?: string }): never {
  throw new Error(message);
}

const flaky = { failing: true };

function Flaky() {
  if (flaky.failing) {
    throw new Error('transient');
  }
  return <p>recovered</p>;
}

beforeEach(() => {
  // React and the boundary both log a caught render error; that is the
  // behaviour under test, not noise worth printing.
  vi.spyOn(console, 'error').mockImplementation(() => {});
});
afterEach(() => vi.restoreAllMocks());

describe('ErrorBoundary', () => {
  it('shows a fallback that names the product instead of a white screen', () => {
    render(
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>,
    );

    const fallback = screen.getByTestId('error-boundary-fallback');
    expect(fallback).toHaveTextContent('Trellis hit an unexpected error');
    expect(screen.getByRole('alert')).toBe(fallback);
  });

  it('offers a reload and a way home that needs no router', () => {
    const reload = vi.fn();
    vi.stubGlobal('location', { ...window.location, reload });
    render(
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>,
    );

    expect(screen.getByTestId('error-boundary-home')).toHaveAttribute('href', '/');
    fireEvent.click(screen.getByTestId('error-boundary-reload'));
    expect(reload).toHaveBeenCalledTimes(1);
    vi.unstubAllGlobals();
  });

  it('logs the error with its component stack for whoever reports it', () => {
    render(
      <ErrorBoundary>
        <Boom message="specific failure" />
      </ErrorBoundary>,
    );

    expect(console.error).toHaveBeenCalledWith(
      'render error',
      expect.objectContaining({ message: 'specific failure' }),
      expect.any(String),
    );
  });

  it('renders its children when nothing throws', () => {
    render(
      <ErrorBoundary>
        <p>fine</p>
      </ErrorBoundary>,
    );
    expect(screen.getByText('fine')).toBeInTheDocument();
    expect(screen.queryByTestId('error-boundary-fallback')).not.toBeInTheDocument();
  });
});

describe('PageErrorBoundary', () => {
  it('keeps the failure to the page and recovers on retry', () => {
    flaky.failing = true;

    render(
      <>
        <nav data-testid="rail">rail</nav>
        <PageErrorBoundary>
          <Flaky />
        </PageErrorBoundary>
      </>,
    );

    expect(screen.getByTestId('page-error-boundary-fallback')).toHaveTextContent(
      'This page failed to load',
    );
    expect(screen.getByTestId('rail')).toBeInTheDocument();

    flaky.failing = false;
    fireEvent.click(screen.getByTestId('page-error-boundary-retry'));
    expect(screen.getByText('recovered')).toBeInTheDocument();
  });

  it('does not carry the error to another page when keyed by path', () => {
    const { rerender } = render(
      <PageErrorBoundary key="/coverage">
        <Boom />
      </PageErrorBoundary>,
    );
    expect(screen.getByTestId('page-error-boundary-fallback')).toBeInTheDocument();

    rerender(
      <PageErrorBoundary key="/live">
        <p>live page</p>
      </PageErrorBoundary>,
    );
    expect(screen.getByText('live page')).toBeInTheDocument();
    expect(screen.queryByTestId('page-error-boundary-fallback')).not.toBeInTheDocument();
  });
});
