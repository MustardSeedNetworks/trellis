import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ToastProvider } from './Toast';
import { useToast } from './toastContext';

function Trigger({ type, duration }: { type?: 'success' | 'error'; duration?: number }) {
  const { addToast } = useToast();
  return (
    <button type="button" onClick={() => addToast('Saved the floor', type, duration)}>
      go
    </button>
  );
}

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe('toasts', () => {
  it('shows a toast as an alert inside a labelled live region', () => {
    render(
      <ToastProvider>
        <Trigger type="error" />
      </ToastProvider>,
    );

    fireEvent.click(screen.getByText('go'));

    expect(screen.getByRole('alert')).toHaveTextContent('Saved the floor');
    expect(screen.getByTestId('toast')).toHaveAttribute('data-toast-type', 'error');
    expect(screen.getByLabelText('Notifications')).toContainElement(screen.getByRole('alert'));
  });

  it('dismisses itself after its duration', () => {
    render(
      <ToastProvider>
        <Trigger duration={3000} />
      </ToastProvider>,
    );
    fireEvent.click(screen.getByText('go'));

    act(() => vi.advanceTimersByTime(2999));
    expect(screen.getByRole('alert')).toBeInTheDocument();
    act(() => vi.advanceTimersByTime(1));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('stays until dismissed when the duration is zero, and queues several', () => {
    render(
      <ToastProvider>
        <Trigger duration={0} />
      </ToastProvider>,
    );
    fireEvent.click(screen.getByText('go'));
    fireEvent.click(screen.getByText('go'));
    act(() => vi.advanceTimersByTime(60_000));

    expect(screen.getAllByRole('alert')).toHaveLength(2);
    fireEvent.click(screen.getAllByTestId('toast-dismiss')[0] as HTMLElement);
    expect(screen.getAllByRole('alert')).toHaveLength(1);
  });

  it('refuses to be used without a provider', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {});
    expect(() => render(<Trigger />)).toThrow('useToast must be used within a ToastProvider');
    vi.restoreAllMocks();
  });
});
