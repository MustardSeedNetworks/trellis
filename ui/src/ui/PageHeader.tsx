/**
 * PageHeader — page-level title bar with optional breadcrumbs, actions,
 * and a help entry point.
 *
 * Shared shell pattern — kept visually and behaviorally consistent across
 * seed / stem / niac by convention; each repo owns this file independently
 * (no master, no sync). All colors/spacing reference theme tokens.
 *
 * Usage:
 *   <PageHeader
 *     title="Devices"
 *     description="Active simulated devices in this NIAC instance"
 *     icon={ServerIcon}
 *     breadcrumbs={[{ label: 'Home', href: '/' }, { label: 'Devices' }]}
 *     actions={<Button>Add device</Button>}
 *     onHelp={() => openHelp('devices')}
 *   />
 */
import type { LucideIcon } from 'lucide-react';
import { ChevronRight, HelpCircle } from 'lucide-react';
import { createElement, type FC, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { iconSizes } from '../constants/sizes';
import { Tooltip } from './Tooltip';

interface BreadcrumbItem {
  label: string;
  href?: string;
}

interface PageHeaderProps {
  title: string;
  /**
   * Kicker above the title naming the product domain — "Reflector", "Benchmark".
   * Optional so existing callers are unaffected.
   */
  eyebrow?: string;
  /**
   * The slot beside the primary action. Prefer a state readout to a second
   * button: "Uptime 02:41:19", "Last poll 41s", "64 targets". A page with no
   * single most-likely action should have no primary button rather than an
   * invented one.
   */
  secondary?: ReactNode;
  description?: string;
  icon?: LucideIcon;
  iconColorClass?: string;
  actions?: ReactNode;
  breadcrumbs?: BreadcrumbItem[];
  /**
   * Opens the application's help drawer on this page's section. The (?)
   * button renders only when this is supplied — a page with no help
   * content in the drawer shows no entry point to it.
   */
  onHelp?: () => void;
  className?: string;
}

interface BreadcrumbProps {
  items: BreadcrumbItem[];
  className?: string;
}

const Breadcrumb: FC<BreadcrumbProps> = ({ items, className = '' }) => {
  const { t } = useTranslation('common');

  return (
    <nav
      className={`flex items-center gap-tight text-sm ${className}`}
      aria-label={t('accessibility.breadcrumb')}
    >
      {items.map((item, index) => (
        <div key={item.label} className="flex items-center gap-tight">
          {index > 0 && <ChevronRight className={`${iconSizes.md} text-text-disabled`} />}
          {item.href ? (
            <Link
              to={item.href}
              className="text-text-muted hover:text-text-primary transition-colors"
            >
              {item.label}
            </Link>
          ) : (
            <span className="text-text-secondary font-medium">{item.label}</span>
          )}
        </div>
      ))}
    </nav>
  );
};

export const PageHeader: FC<PageHeaderProps> = ({
  title,
  eyebrow,
  secondary,
  description,
  icon,
  iconColorClass = 'text-brand-primary',
  actions,
  breadcrumbs,
  onHelp,
  className = '',
}) => {
  const { t } = useTranslation('common');

  return (
    <div className={`mb-content ${className}`} data-testid="page-header">
      {breadcrumbs && breadcrumbs.length > 0 && (
        <Breadcrumb items={breadcrumbs} className="mb-tight" />
      )}
      <div className="flex flex-wrap items-start justify-between gap-default">
        <div className="flex items-center gap-default">
          {icon ? createElement(icon, { className: `h-6 w-6 ${iconColorClass}` }) : null}
          <div>
            {eyebrow ? (
              <p className="kicker mb-1" data-testid="page-header-eyebrow">
                {eyebrow}
              </p>
            ) : null}
            <h1 className="heading-1 font-display" data-testid="page-header-title">
              {title}
            </h1>
            {description ? <p className="body-small mt-tight max-w-2xl">{description}</p> : null}
          </div>
        </div>
        <div className="flex items-center gap-default">
          {secondary ? (
            <div className="figure text-sm text-text-secondary" data-testid="page-header-secondary">
              {secondary}
            </div>
          ) : null}
          {actions}
          {onHelp ? (
            <Tooltip content={t('accessibility.whatIs', { title })}>
              {(tooltip) => (
                <button
                  {...tooltip}
                  type="button"
                  data-testid="page-help-open"
                  onClick={(event) => {
                    event.currentTarget.focus();
                    onHelp();
                  }}
                  aria-label={t('accessibility.openHelp', { title })}
                  className="flex min-h-11 min-w-11 items-center justify-center rounded-full text-text-muted hover:bg-surface-hover hover:text-text-primary"
                >
                  <HelpCircle aria-hidden="true" className={iconSizes.lg} />
                </button>
              )}
            </Tooltip>
          ) : null}
        </div>
      </div>
    </div>
  );
};
