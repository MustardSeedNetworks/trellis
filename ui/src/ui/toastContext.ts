import { createContext, use } from 'react';

export type ToastType = 'success' | 'error' | 'warning' | 'info';

export interface ToastApi {
  /** Shows message until duration ms pass (0 keeps it until dismissed). */
  addToast: (message: string, type?: ToastType, duration?: number) => void;
  removeToast: (id: number) => void;
}

export const ToastContext = createContext<ToastApi | null>(null);

/** useToast is for failures and confirmations that have no field to attach to. */
export function useToast(): ToastApi {
  const api = use(ToastContext);
  if (!api) {
    throw new Error('useToast must be used within a ToastProvider');
  }
  return api;
}
