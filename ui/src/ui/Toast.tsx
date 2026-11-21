import { AlertCircle, CheckCircle2, Info, TriangleAlert, X } from 'lucide-react';
import { type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { iconSizes } from '../constants/sizes';
import { ToastContext, type ToastType } from './toastContext';

const DEFAULT_DURATION_MS = 5_000;

interface ToastEntry {
  id: number;
  message: string;
  type: ToastType;
  duration: number;
}

const EDGE: Record<ToastType, string> = {
  success: 'border-l-status-success',
  error: 'border-l-status-error',
  warning: 'border-l-status-warning',
  info: 'border-l-brand-primary',
};

const ICON: Record<ToastType, string> = {
  success: 'text-status-success',
  error: 'text-status-error',
  warning: 'text-status-warning',
  info: 'text-brand-primary',
};

function ToastIcon({ type }: { type: ToastType }) {
  const className = `${iconSizes.lg} shrink-0 ${ICON[type]}`;
  switch (type) {
    case 'success':
      return <CheckCircle2 className={className} aria-hidden />;
    case 'error':
      return <AlertCircle className={className} aria-hidden />;
    case 'warning':
      return <TriangleAlert className={className} aria-hidden />;
    case 'info':
      return <Info className={className} aria-hidden />;
  }
}

/**
 * ToastProvider owns the queue and draws it bottom-right, above everything
 * else but the command palette. Wrap the app once; any component below calls
 * useToast().
 */
export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastEntry[]>([]);
  const nextId = useRef(0);

  const removeToast = useCallback((id: number) => {
    setToasts((current) => current.filter((toast) => toast.id !== id));
  }, []);

  const addToast = useCallback(
    (message: string, type: ToastType = 'info', duration = DEFAULT_DURATION_MS) => {
      const id = nextId.current++;
      setToasts((current) => [...current, { id, message, type, duration }]);
    },
    [],
  );

  const api = useMemo(() => ({ addToast, removeToast }), [addToast, removeToast]);

  return (
    <ToastContext value={api}>
      {children}
      <ToastRegion toasts={toasts} onClose={removeToast} />
    </ToastContext>
  );
}

function ToastRegion({ toasts, onClose }: { toasts: ToastEntry[]; onClose: (id: number) => void }) {
  const { t } = useTranslation('common');

  return (
    <section
      aria-live="polite"
      aria-label={t('accessibility.notifications')}
      className="pointer-events-none fixed right-4 bottom-4 z-50 flex w-[calc(100%-2rem)] max-w-sm flex-col gap-2"
    >
      {toasts.map((toast) => (
        <ToastItem key={toast.id} toast={toast} onClose={onClose} />
      ))}
    </section>
  );
}

function ToastItem({ toast, onClose }: { toast: ToastEntry; onClose: (id: number) => void }) {
  const { t } = useTranslation('common');
  const { id, duration } = toast;

  useEffect(() => {
    if (duration <= 0) {
      return;
    }
    const timer = setTimeout(() => onClose(id), duration);
    return () => clearTimeout(timer);
  }, [id, duration, onClose]);

  return (
    <div
      role="alert"
      data-testid="toast"
      data-toast-type={toast.type}
      className={`panel pointer-events-auto flex items-start gap-3 border-l-4 p-3 text-sm text-text-primary ${EDGE[toast.type]}`}
    >
      <ToastIcon type={toast.type} />
      <p className="min-w-0 flex-1 break-words">{toast.message}</p>
      <button
        type="button"
        onClick={() => onClose(id)}
        data-testid="toast-dismiss"
        aria-label={t('accessibility.dismissNotification')}
        className="-m-1 flex min-h-8 min-w-8 items-center justify-center rounded text-text-muted hover:bg-surface-hover hover:text-text-primary"
      >
        <X className={iconSizes.md} aria-hidden />
      </button>
    </div>
  );
}
