/**
 * Sidebar — the persistent left rail.
 *
 * Shared shell pattern, kept visually and behaviourally consistent across
 * seed / stem / niac / trellis by convention; each repo owns its own copy (no
 * master, no sync). SHELL.md carries the conventions this implements: 252px on
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
import { ChevronsLeft, ChevronsRight, Settings } from 'lucide-react';
import { type FC, useEffect, useState } from 'react';
import { NavLink } from 'react-router';
import { iconSizes } from '../constants/sizes';
import { navGroups } from '../navGroups';
import { MsnMark } from './MsnMark';

const STORAGE_KEY = 'trellis-sidebar-collapsed';

interface SidebarProps {
  version?: string;
  onOpenSettings?: () => void;
}

export const Sidebar: FC<SidebarProps> = ({ version, onOpenSettings }) => {
  const [collapsed, setCollapsed] = useState(() => localStorage.getItem(STORAGE_KEY) === 'true');

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, String(collapsed));
  }, [collapsed]);

  return (
    <aside
      className={`flex h-full flex-col border-r border-hairline bg-gradient-to-b from-rail-from to-rail-to transition-all duration-300 ease-in-out ${
        collapsed ? 'w-16' : 'w-[252px]'
      }`}
    >
      {/* Brand lockup. Two characters, never one: "T" for Stem and "R" for
          Trellis both read as typos next to the other products. */}
      <div
        className={`flex items-center gap-2 border-b border-hairline px-3 py-4 ${
          collapsed ? 'justify-center' : ''
        }`}
      >
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[11px] bg-brand-primary">
          <span className="figure text-sm font-extrabold tracking-tight text-on-brand">TR</span>
        </div>
        {!collapsed ? (
          <span className="truncate font-extrabold tracking-tight text-text-primary">Trellis</span>
        ) : null}
      </div>

      <nav className="flex-1 overflow-y-auto px-3 py-4" aria-label="Trellis sections">
        {navGroups.map((group) => (
          <div key={group.label} className="mb-6">
            {!collapsed ? <p className="kicker mb-2 px-3">{group.label}</p> : null}
            <ul className="space-y-1">
              {group.items.map((item) => (
                <li key={item.path}>
                  <NavLink
                    to={item.path}
                    end={item.path === '/'}
                    title={collapsed ? item.label : undefined}
                    className={({ isActive }) =>
                      `group relative flex min-h-11 w-full items-center gap-3 rounded-[11px] px-3 py-2.5 text-sm font-medium transition-all duration-200 ${
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
                </li>
              ))}
            </ul>
          </div>
        ))}
      </nav>

      <div className={`border-t border-hairline px-3 py-4 ${collapsed ? 'text-center' : ''}`}>
        {onOpenSettings ? (
          <button
            type="button"
            onClick={onOpenSettings}
            title="Settings"
            className={`mb-3 flex min-h-11 items-center gap-2 rounded-[11px] px-3 text-sm text-text-muted hover:bg-surface-hover hover:text-text-primary ${
              collapsed ? 'w-full justify-center' : 'w-full'
            }`}
          >
            <Settings className={iconSizes.md} />
            {!collapsed ? <span>Settings</span> : null}
          </button>
        ) : null}

        {version ? (
          <div
            className={`figure text-xs text-text-muted ${collapsed ? '' : 'flex items-center justify-between'}`}
          >
            {!collapsed ? <span>Version</span> : null}
            <span>{version}</span>
          </div>
        ) : null}

        {/* Whose tool this is, under what it is. Quiet by design: the product
            mark at the top of the rail is the one that has to be recognised. */}
        <MsnMark collapsed={collapsed} className="mt-3" />

        <button
          type="button"
          onClick={() => setCollapsed((value) => !value)}
          aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          className="mt-3 flex min-h-11 w-full items-center justify-center rounded-[11px] text-text-muted hover:bg-surface-hover hover:text-text-primary"
        >
          {collapsed ? (
            <ChevronsRight className={iconSizes.md} />
          ) : (
            <ChevronsLeft className={iconSizes.md} />
          )}
        </button>
      </div>
    </aside>
  );
};
