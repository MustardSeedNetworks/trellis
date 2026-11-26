/**
 * jsdom implements no `ResizeObserver`, and the shared `useScrollable` (every
 * DataTable and the Modal body) and cmdk both construct one. This is the stub
 * every test gets: typed with the real constructor signature, so the call
 * `new ResizeObserver(callback)` matches it. It never reports a size, because
 * jsdom has no layout to report.
 */
import { vi } from 'vitest';

class ResizeObserverStub implements ResizeObserver {
  readonly callback: ResizeObserverCallback;

  constructor(callback: ResizeObserverCallback) {
    this.callback = callback;
  }

  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}

export function installResizeObserverStub(): void {
  vi.stubGlobal('ResizeObserver', ResizeObserverStub);
}
