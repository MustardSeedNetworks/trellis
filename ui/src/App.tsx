import type { ReactNode } from 'react';
import { Suspense, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Route, Routes, useLocation } from 'react-router';
import { PageErrorBoundary } from '@/components/ErrorBoundary';
import { useBuildVersion } from '@/hooks/useBuildVersion';
import { useConnectionStatus } from '@/hooks/useConnectionStatus';
import { type PageConfig, usePages } from '@/pageRegistry';
import type { BreadcrumbItem, BreadcrumbLinkProps } from '@/shared/Breadcrumbs';
import { ButtonLink } from '@/shared/Button';
import { Card } from '@/shared/Card';
import { documentTitle } from '@/shared/documentTitle';
import { EmptyState } from '@/shared/EmptyState';
import { PageHeader } from '@/shared/PageHeader';
import { RightDrawer } from '@/shared/RightDrawer';
import { CommandPalette } from '@/ui/CommandPalette';
import { DaemonUnreachable } from '@/ui/DaemonUnreachable';
import { MobileNav } from '@/ui/MobileNav';
import { SettingsDrawer } from '@/ui/SettingsDrawer';
import { Sidebar } from '@/ui/Sidebar';

/**
 * Shell + routes. The rail, page header and status rollup are the family shell
 * (see ui/SHELL.md in the sibling products); everything inside a route is
 * trellis's own.
 *
 * Every rail entry has a page, so an unmatched path is a mistyped URL or a
 * stale link rather than a feature on its way — and it says so. The
 * placeholder that used to answer here told a reader that /interferance was
 * being built.
 */
export function App() {
  const pages = usePages();
  const { t } = useTranslation('common');
  const { pathname } = useLocation();
  const path = pathname.replace(/\/+$/, '') || '/';
  const page = pages.find((entry) => entry.path === path);
  const [helpPath, setHelpPath] = useState<string | null>(null);
  const title = page?.title ?? t('emptyState.notFoundTitle');
  const build = useBuildVersion();
  const { version } = build;
  const [settingsOpen, setSettingsOpen] = useState(false);
  const connection = useConnectionStatus();

  useEffect(() => {
    setHelpPath((openPath) => (openPath === path ? openPath : null));
  }, [path]);

  useEffect(() => {
    document.title = documentTitle(title, 'Trellis');
  }, [title]);

  return (
    <div className="flex h-screen flex-col bg-surface-base text-text-primary lg:flex-row">
      <a
        href="#main-content"
        data-testid="skip-to-content"
        className="fixed left-2 top-0 z-50 -translate-y-full rounded bg-surface-raised p-3 focus:translate-y-2"
      >
        {t('accessibility.skipToContent')}
      </a>
      <MobileNav version={version} onOpenSettings={() => setSettingsOpen(true)} />
      <Sidebar version={version} onOpenSettings={() => setSettingsOpen(true)} />
      <CommandPalette />
      <main
        id="main-content"
        tabIndex={-1}
        className="flex min-h-0 flex-1 flex-col overflow-hidden"
      >
        {connection === 'disconnected' ? <DaemonUnreachable /> : null}
        <Routes>
          {pages.map((page) => (
            <Route
              key={page.path}
              path={page.path}
              element={
                <PageFrame page={page} onHelp={() => setHelpPath(page.path)}>
                  <page.component />
                </PageFrame>
              }
            />
          ))}
          <Route path="*" element={<NotFound />} />
        </Routes>
      </main>
      <RightDrawer
        open={page !== undefined && helpPath === path}
        onClose={() => setHelpPath(null)}
        title={t('accessibility.helpTitle', { title })}
        closeLabel={t('accessibility.closeHelp')}
        data-testid="page-help-drawer"
      >
        <p className="text-sm leading-relaxed text-text-secondary" data-testid="page-help-copy">
          {page?.help}
        </p>
      </RightDrawer>
      <SettingsDrawer open={settingsOpen} onClose={() => setSettingsOpen(false)} build={build} />
    </div>
  );
}

/**
 * PageFrame holds what every routed page shares: the one `PageHeader`, built
 * from the registry entry rather than the page body, then the page. The frame
 * scrolls; a page that must stay inside the viewport (Coverage) bounds itself.
 */
function PageFrame({
  page,
  children,
  onHelp,
}: {
  page: PageConfig;
  children: ReactNode;
  onHelp: () => void;
}) {
  const { t } = useTranslation('common');

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto p-4 sm:p-6">
      <PageHeader
        kicker={page.kicker}
        title={page.title}
        subtitle={page.description}
        breadcrumbs={trail(page.title)}
        breadcrumbsLabel={t('accessibility.breadcrumb')}
        renderBreadcrumbLink={renderBreadcrumbLink}
        help={{ label: t('accessibility.openHelp', { title: page.title }), onClick: onHelp }}
      />
      {/* Keyed by path so a page that threw does not stay broken after the
          operator navigates somewhere else; the header above stays up either
          way. */}
      <PageErrorBoundary key={page.path}>
        <Suspense fallback={<PageLoading />}>{children}</Suspense>
      </PageErrorBoundary>
    </div>
  );
}

/** Trellis, then this page: the trail every page carries, ending on itself. */
function trail(title: string): BreadcrumbItem[] {
  return [{ label: 'Trellis', href: '/' }, { label: title }];
}

function renderBreadcrumbLink({ href, className, children }: BreadcrumbLinkProps) {
  return (
    <Link to={href} className={className}>
      {children}
    </Link>
  );
}

/**
 * Fallback shown while a lazily-loaded page's chunk is still in flight.
 */
function PageLoading() {
  const { t } = useTranslation('common');

  return (
    <div className="flex flex-1 items-center justify-center p-8 text-sm text-text-secondary">
      {t('loading.page')}
    </div>
  );
}

/**
 * Says what it is rather than pretending: this address is not a page. It wears
 * the same header as every other page, so it reads as Trellis answering rather
 * than as a blank pane.
 */
function NotFound() {
  const { t } = useTranslation('common');
  const title = t('emptyState.notFoundTitle');

  return (
    <div
      className="flex min-h-0 flex-1 flex-col overflow-y-auto p-4 sm:p-6"
      data-testid="not-found"
    >
      <PageHeader
        kicker={t('emptyState.notFoundKicker')}
        title={title}
        breadcrumbs={trail(title)}
        breadcrumbsLabel={t('accessibility.breadcrumb')}
        renderBreadcrumbLink={renderBreadcrumbLink}
      />
      <Card>
        <EmptyState
          title={title}
          hint={t('emptyState.notFoundBody')}
          action={
            <ButtonLink variant="primary" renderLink={(link) => <Link to="/" {...link} />}>
              {t('emptyState.notFoundHome')}
            </ButtonLink>
          }
        />
      </Card>
    </div>
  );
}
