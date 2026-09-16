import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { HelpDrawer } from './HelpDrawer';

const showModal = vi.fn(function (this: HTMLDialogElement) {
  this.open = true;
});
const close = vi.fn(function (this: HTMLDialogElement) {
  this.open = false;
  this.dispatchEvent(new Event('close'));
});

beforeEach(() => {
  // jsdom has neither native method. Real modality and restoration run in Playwright.
  Object.defineProperties(HTMLDialogElement.prototype, {
    showModal: { configurable: true, value: showModal },
    close: { configurable: true, value: close },
  });
  vi.clearAllMocks();
});

afterEach(() => {
  cleanup();
  Reflect.deleteProperty(HTMLDialogElement.prototype, 'showModal');
  Reflect.deleteProperty(HTMLDialogElement.prototype, 'close');
});

describe('HelpDrawer', () => {
  it('opens one labelled modal with the route guidance and closes through its control', () => {
    const onClose = vi.fn();
    render(
      <HelpDrawer title="Surveys" content="Walk a floor to capture samples." onClose={onClose} />,
    );
    expect(showModal).toHaveBeenCalledOnce();
    expect(screen.getByRole('dialog', { name: 'Help: Surveys' })).toHaveTextContent('Walk a floor');
    fireEvent.click(screen.getByRole('button', { name: 'Close help' }));
    expect(close).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('keeps Tab on its only control and closes the native dialog on unmount', () => {
    const { unmount } = render(
      <HelpDrawer title="Live" content="Nearby networks." onClose={vi.fn()} />,
    );
    const dialog = screen.getByRole('dialog');
    fireEvent.keyDown(dialog, { key: 'Tab', shiftKey: true });
    expect(screen.getByRole('button', { name: 'Close help' })).toHaveFocus();
    fireEvent.keyDown(dialog, { key: 'ArrowDown' });
    expect(close).not.toHaveBeenCalled();
    unmount();
    expect(close).toHaveBeenCalledOnce();
  });
});
