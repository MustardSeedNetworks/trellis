import { Menu, X } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation } from 'react-router';
import { iconSizes } from '../constants/sizes';
import { Sidebar } from './Sidebar';

/**
 * Below lg the rail is not drawn. This puts a top bar in its place and opens
 * the rail's contents as a slide-out drawer, so a phone keeps the whole width
 * for the page. It closes on any navigation.
 */
export function MobileNav({ version }: { version?: string }) {
  const { t } = useTranslation('common');
  const { pathname } = useLocation();
  // The drawer is open for the path it was opened on, so any navigation —
  // including browser back and forward, which this component does not start —
  // closes it without an effect that has to remember to. A tap on the page
  // already open changes no path; onNavigate covers that one.
  const [openOn, setOpenOn] = useState<string | null>(null);
  const open = openOn === pathname;
  const close = () => setOpenOn(null);
  const label = open ? t('accessibility.closeMenu') : t('accessibility.openMenu');

  useEffect(() => {
    if (!open) {
      return;
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpenOn(null);
      }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [open]);

  return (
    <div className="lg:hidden">
      <header
        data-testid="mobile-top-bar"
        className="flex shrink-0 items-center justify-between border-b border-hairline bg-surface-raised px-4 py-2"
      >
        <div className="flex items-center gap-2">
          <div
            aria-hidden="true"
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[11px] bg-brand-primary"
          >
            <span className="figure text-xs font-extrabold tracking-tight text-on-brand">TR</span>
          </div>
          {/* allow-hardcoded: product name, not copy. */}
          <span className="font-extrabold tracking-tight text-text-primary">Trellis</span>
        </div>
        <button
          type="button"
          onClick={() => setOpenOn(open ? null : pathname)}
          data-testid="mobile-menu-toggle"
          aria-label={label}
          aria-expanded={open}
          className="flex min-h-11 min-w-11 items-center justify-center rounded text-text-muted hover:bg-surface-hover hover:text-text-primary"
        >
          {open ? <X className={iconSizes.lg} /> : <Menu className={iconSizes.lg} />}
        </button>
      </header>

      {open ? (
        <button
          type="button"
          data-testid="mobile-scrim"
          aria-label={t('accessibility.closeMenu')}
          onClick={close}
          className="fixed inset-0 z-40 bg-scrim/60"
        />
      ) : null}

      {/* The panel stays in the tree for the slide and is `invisible` when
          closed, so a parked drawer never paints outside the viewport. Its
          contents mount only while open: a closed drawer would otherwise hold a
          second copy of every control in a menu nobody can see. */}
      <div
        className={`fixed inset-y-0 left-0 z-50 transition-[transform,visibility] duration-300 ease-in-out motion-reduce:transition-none ${
          open ? 'translate-x-0' : 'invisible -translate-x-full'
        }`}
      >
        {open ? <Sidebar variant="drawer" version={version} onNavigate={close} /> : null}
      </div>
    </div>
  );
}
