import { X } from 'lucide-react';
import { useEffect, useId, useRef } from 'react';
import { useTranslation } from 'react-i18next';

export function HelpDrawer({
  title,
  content,
  onClose,
}: {
  title: string;
  content: string;
  onClose: () => void;
}) {
  const { t } = useTranslation('common');
  const titleId = useId();
  const dialog = useRef<HTMLDialogElement>(null);
  const closeButton = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    const element = dialog.current;
    element?.showModal();
    return () => element?.close();
  }, []);

  return (
    <dialog
      ref={dialog}
      aria-labelledby={titleId}
      onClose={onClose}
      onKeyDown={(event) => {
        if (event.key === 'Tab') {
          event.preventDefault();
          closeButton.current?.focus();
        }
      }}
      data-testid="page-help-drawer"
      className="fixed inset-y-0 left-auto right-0 m-0 h-dvh max-h-none w-full max-w-md border-l border-hairline bg-surface-base p-6 text-text-primary shadow-xl backdrop:bg-scrim/60"
    >
      <div className="flex items-start justify-between gap-4">
        <h2 id={titleId} className="font-display text-lg font-semibold">
          {t('accessibility.helpTitle', { title })}
        </h2>
        <button
          ref={closeButton}
          type="button"
          data-testid="page-help-close"
          aria-label={t('accessibility.closeHelp')}
          onClick={() => dialog.current?.close()}
          className="flex min-h-11 min-w-11 shrink-0 items-center justify-center rounded-full text-text-muted hover:bg-surface-hover hover:text-text-primary"
        >
          <X aria-hidden="true" className="h-5 w-5" />
        </button>
      </div>
      <p className="mt-4 text-sm leading-relaxed text-text-secondary" data-testid="page-help-copy">
        {content}
      </p>
    </dialog>
  );
}
