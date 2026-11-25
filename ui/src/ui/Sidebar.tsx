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
import { ChevronsLeft, ChevronsRight, LogOut, Moon, Settings, Sun } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { NavLink } from 'react-router';
import { Tooltip, type TooltipProps } from '@/shared/Tooltip';
import { useAuth } from '@/ui/AuthGate';
import { iconSizes } from '../constants/sizes';
import { useTheme } from '../hooks/useTheme';
import { type TrellisNavItem, useNavGroups } from '../navGroups';
import { MsnMark } from './MsnMark';
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
  /** Opens the Settings drawer. The rail only asks; the shell owns the drawer. */
  onOpenSettings: () => void;
}

export function Sidebar({ version, variant = 'rail', onNavigate, onOpenSettings }: SidebarProps) {
  const { t } = useTranslation('common');
  const navGroups = useNavGroups();
  const drawer = variant === 'drawer';
  const [chosenCollapsed, setChosenCollapsed] = useState(
    () => localStorage.getItem(STORAGE_KEY) === 'true',
  );

  const collapsed = !drawer && chosenCollapsed;

  useEffect(() => {
    if (!drawer) {
      localStorage.setItem(STORAGE_KEY, String(chosenCollapsed));
    }
  }, [chosenCollapsed, drawer]);

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

      {/* The collapsed rail keeps a narrower gutter than the expanded one: at
          px-3 the version number is 43px of text in a 39px box and clips. */}
      <div className={`border-t border-hairline py-4 ${collapsed ? 'px-1 text-center' : 'px-3'}`}>
        {version ? (
          <div
            className={`figure text-xs text-text-muted ${collapsed ? '' : 'flex items-center justify-between'}`}
          >
            {!collapsed ? <span>{t('labels.version')}</span> : null}
            <span>{version}</span>
          </div>
        ) : null}

        {/* Whose tool this is, under what it is. Quiet by design: the product
            mark at the top of the rail is the one that has to be recognised. */}
        <MsnMark collapsed={collapsed} className="mt-3" />

        <SettingsButton collapsed={collapsed} onClick={onOpenSettings} />

        <ThemeToggle collapsed={collapsed} />

        <SignOutButton collapsed={collapsed} />

        {/* The rail is not drawn below lg, so a phone never sees this: expanding
            a rail to 224px there left 166px of page, the defect that made the
            drawer necessary. */}
        {!drawer ? (
          <RailTip
            text={collapsed ? t('accessibility.expandSidebar') : t('accessibility.collapseSidebar')}
          >
            <button
              type="button"
              onClick={() => setChosenCollapsed((value) => !value)}
              data-testid="sidebar-collapse"
              aria-label={
                collapsed ? t('accessibility.expandSidebar') : t('accessibility.collapseSidebar')
              }
              className="mt-3 flex min-h-11 w-full items-center justify-center rounded-[11px] text-text-muted hover:bg-surface-hover hover:text-text-primary"
            >
              {collapsed ? (
                <ChevronsRight className={iconSizes.md} />
              ) : (
                <ChevronsLeft className={iconSizes.md} />
              )}
            </button>
          </RailTip>
        ) : null}
      </div>
    </aside>
  );
}

const RAIL_BUTTON =
  'mt-3 flex min-h-11 w-full items-center justify-center gap-2 rounded-[11px] text-text-muted hover:bg-surface-hover hover:text-text-primary';

/**
 * The shared Tooltip wraps its trigger in an inline-flex span, which would
 * shrink a full-width rail button to its content; this stretches that span back.
 */
function RailTip({ text, children }: { text: string; children: TooltipProps['children'] }) {
  return (
    <div className="[&>span]:w-full">
      <Tooltip text={text}>{children}</Tooltip>
    </div>
  );
}

/** Opens the Settings drawer, where the theme, the language and the build details live. */
function SettingsButton({ collapsed, onClick }: { collapsed: boolean; onClick: () => void }) {
  const { t } = useTranslation('common');
  const button = (
    <button
      type="button"
      // Safari does not focus a clicked button; focusing it is what lets the
      // drawer give focus back here when it closes.
      onClick={(event) => {
        event.currentTarget.focus();
        onClick();
      }}
      data-testid="open-settings"
      aria-label={t('settings.title')}
      className={RAIL_BUTTON}
    >
      <Settings className={iconSizes.md} />
      {!collapsed ? <span className="text-xs">{t('settings.title')}</span> : null}
    </button>
  );
  return collapsed ? <RailTip text={t('settings.title')}>{button}</RailTip> : button;
}

/**
 * The quick theme toggle: light and dark, one click. It is on the same store
 * as the Settings control (System / Light / Dark), so choosing in either shows
 * in both. Its label names the current choice in the Settings control's words;
 * the tooltip says where one click goes.
 */
function ThemeToggle({ collapsed }: { collapsed: boolean }) {
  const { t } = useTranslation('common');
  const { theme, isDark, toggleTheme } = useTheme();
  const choice = {
    system: t('settings.themeSystem'),
    light: t('settings.themeLight'),
    dark: t('settings.themeDark'),
  }[theme];
  const label = t('settings.themeQuick', { theme: choice });
  const destination = isDark
    ? t('accessibility.switchToLightTheme')
    : t('accessibility.switchToDarkTheme');

  return (
    <RailTip text={destination}>
      <button
        type="button"
        onClick={toggleTheme}
        data-testid="theme-toggle"
        aria-label={label}
        className={RAIL_BUTTON}
      >
        {isDark ? <Sun className={iconSizes.md} /> : <Moon className={iconSizes.md} />}
        {!collapsed ? <span className="text-xs">{label}</span> : null}
      </button>
    </RailTip>
  );
}

/**
 * Ends the operator's session. Rendered only on a daemon that asked for one:
 * the loopback desktop app has no session to sign out of, and an inert control
 * there would read as a broken one.
 */
function SignOutButton({ collapsed }: { collapsed: boolean }) {
  const { t } = useTranslation('pages');
  const { required, signOut } = useAuth();

  if (!required) {
    return null;
  }
  return (
    <RailTip text={t('login.signOut')}>
      <button
        type="button"
        onClick={signOut}
        data-testid="sign-out"
        aria-label={t('login.signOut')}
        className={RAIL_BUTTON}
      >
        {collapsed ? (
          <LogOut className={iconSizes.md} />
        ) : (
          <span className="text-xs">{t('login.signOut')}</span>
        )}
      </button>
    </RailTip>
  );
}
