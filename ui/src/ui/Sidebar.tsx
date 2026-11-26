/**
 * Sidebar — the persistent left rail.
 *
 * Shared shell pattern, kept visually and behaviourally consistent across
 * seed / stem / niac / trellis by convention; each repo owns its own copy (no
 * master, no sync). SHELL.md carries the conventions this implements: 224px on
 * the rail gradient, a two-character monogram chip, .kicker group labels, and
 * nav items at 44px with an 11px radius and a 3px active bar.
 *
 * This is trellis's own implementation rather than a transplant of stem's. The
 * conventions are identical; the machinery behind them is not — stem's rail
 * carries profile menus, a history drawer and route prefetching that trellis
 * has no equivalent of yet. Copying those in to look the same would have added
 * four unused surfaces, and the rule this family keeps is that the shell is
 * shared while each product's own contents are not.
 */
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { NavLink } from 'react-router';
import { RailFooter } from '@/shared/RailFooter';
import type { StatusTone } from '@/shared/StatusPill';
import { Tooltip } from '@/shared/Tooltip';
import { useAuth } from '@/ui/AuthGate';
import msnLogo from '../assets/msn-logo.svg';
import { iconSizes } from '../constants/sizes';
import type { ConnectionState } from '../hooks/useConnectionStatus';
import { useTheme } from '../hooks/useTheme';
import { type TrellisNavItem, useNavGroups } from '../navGroups';
import { ProductMark } from './ProductMark';

const STORAGE_KEY = 'trellis-sidebar-collapsed';

interface SidebarProps {
  version?: string;
  /**
   * The desktop rail, or the same contents as the slide-out drawer MobileNav
   * opens below lg. The drawer is always expanded and has no collapse control:
   * collapsing is a desktop preference, and a phone has no room for it.
   */
  variant?: 'rail' | 'drawer';
  /** Called when a nav link is followed, so the drawer can close on a same-path tap. */
  onNavigate?: () => void;
  /** Whether the daemon answers, from the shell's one probe. */
  connection: ConnectionState;
  /** Opens the current page's help. The rail only asks; the shell owns the drawer. */
  onOpenHelp: () => void;
  /** Opens the Settings drawer. The rail only asks; the shell owns the drawer. */
  onOpenSettings: () => void;
}

const CONNECTION_TONES: Record<ConnectionState, StatusTone> = {
  checking: 'unknown',
  connected: 'success',
  disconnected: 'error',
};

export function Sidebar({
  version,
  variant = 'rail',
  onNavigate,
  connection,
  onOpenHelp,
  onOpenSettings,
}: SidebarProps) {
  const { t } = useTranslation('common');
  const navGroups = useNavGroups();
  const drawer = variant === 'drawer';
  const { t: tPages } = useTranslation('pages');
  const { required, signOut } = useAuth();
  const { isDark, toggleTheme } = useTheme();
  const [chosenCollapsed, setChosenCollapsed] = useState(
    () => localStorage.getItem(STORAGE_KEY) === 'true',
  );

  const collapsed = !drawer && chosenCollapsed;

  useEffect(() => {
    if (!drawer) {
      localStorage.setItem(STORAGE_KEY, String(chosenCollapsed));
    }
  }, [chosenCollapsed, drawer]);

  const statusLabels: Record<ConnectionState, string> = {
    checking: t('rail.status.checking'),
    connected: t('rail.status.connected'),
    disconnected: t('rail.status.disconnected'),
  };

  const link = (item: TrellisNavItem) => (
    <NavLink
      to={item.path}
      onClick={onNavigate}
      end={item.path === '/'}
      aria-label={item.label}
      data-testid={`nav-${item.path === '/' ? 'surveys' : item.path.slice(1)}`}
      className={({ isActive }) =>
        `group relative flex min-h-11 w-full items-center gap-3 rounded-[11px] px-3 py-2.5 text-sm font-medium transition-all duration-200 motion-reduce:transition-none ${
          isActive
            ? 'bg-[color-mix(in_oklab,var(--color-brand-primary)_16%,transparent)] text-text-primary'
            : 'text-text-muted hover:bg-surface-hover hover:text-text-primary'
        }`
      }
    >
      {({ isActive }) => (
        <>
          {isActive ? (
            <span
              aria-hidden="true"
              className="absolute inset-y-1 left-0 w-[3px] rounded-full bg-brand-primary"
            />
          ) : null}
          <item.icon
            className={`${iconSizes.lg} shrink-0 ${
              isActive ? 'text-brand-primary' : 'text-text-muted'
            }`}
          />
          {!collapsed ? <span className="truncate">{item.label}</span> : null}
        </>
      )}
    </NavLink>
  );

  return (
    <aside
      data-testid={drawer ? 'sidebar-mobile' : 'sidebar'}
      className={`h-full flex-col border-r border-hairline bg-gradient-to-b from-rail-from to-rail-to transition-all duration-300 ease-in-out motion-reduce:transition-none ${
        drawer ? 'flex w-72' : `hidden shrink-0 lg:flex ${collapsed ? 'w-16' : 'w-56'}`
      }`}
    >
      {/* Brand lockup. */}
      <div
        className={`flex items-center gap-2 border-b border-hairline px-3 py-4 ${
          collapsed ? 'justify-center' : ''
        }`}
      >
        <ProductMark />
        {/* allow-hardcoded: product name, not copy. */}
        {!collapsed ? (
          <span className="truncate font-extrabold tracking-tight text-text-primary">Trellis</span>
        ) : null}
      </div>

      <nav className="flex-1 overflow-y-auto px-3 py-4" aria-label={t('accessibility.sections')}>
        {navGroups.map((group) => (
          <div key={group.label} className="mb-6">
            {!collapsed ? <p className="kicker mb-2 px-3">{group.label}</p> : null}
            <ul className="space-y-1">
              {group.items.map((item) => (
                <li key={item.path}>
                  {collapsed ? <Tooltip text={item.label}>{link(item)}</Tooltip> : link(item)}
                </li>
              ))}
            </ul>
          </div>
        ))}
      </nav>

      <RailFooter
        collapsed={collapsed}
        variant={variant}
        status={{ tone: CONNECTION_TONES[connection], label: statusLabels[connection] }}
        help={{ label: t('rail.help'), onClick: onOpenHelp }}
        settings={{ label: t('settings.title'), onClick: onOpenSettings }}
        theme={{
          label: isDark
            ? t('accessibility.switchToLightTheme')
            : t('accessibility.switchToDarkTheme'),
          onClick: toggleTheme,
          isDark,
        }}
        {...(required ? { signOut: { label: tPages('login.signOut'), onClick: signOut } } : {})}
        {...(version ? { version: { value: version, label: t('labels.version') } } : {})}
        mark={{ src: msnLogo, name: t('rail.company') }}
        collapse={{
          collapseLabel: t('accessibility.collapseSidebar'),
          expandLabel: t('accessibility.expandSidebar'),
          onToggle: () => setChosenCollapsed((value) => !value),
        }}
      />
    </aside>
  );
}
