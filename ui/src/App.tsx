import type { ReactNode } from 'react';
import { Suspense, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Route, Routes, useLocation } from 'react-router';
import { type PageConfig, usePages } from '@/pageRegistry';
import { HelpDrawer } from '@/ui/HelpDrawer';
import { PageHeader } from '@/ui/PageHeader';
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

  useEffect(() => {
    setHelpPath((openPath) => (openPath === path ? openPath : null));
  }, [path]);

  useEffect(() => {
    document.title = `${title} | Trellis`;
  }, [title]);

  return (
    <div className="flex h-screen bg-surface-base text-text-primary">
      <a
        href="#main-content"
        data-testid="skip-to-content"
        className="fixed left-2 top-0 z-50 -translate-y-full rounded bg-surface-raised p-3 focus:translate-y-2"
      >
        {t('accessibility.skipToContent')}
      </a>
      <Sidebar version={__APP_VERSION__} />
      <main id="main-content" tabIndex={-1} className="flex flex-1 flex-col overflow-hidden">
        <Routes>
          {pages.map((page) => (
            <Route
              key={page.path}
              path={page.path}
              element={
                <PageWithHeader page={page} onHelp={() => setHelpPath(page.path)}>
                  <page.component />
                </PageWithHeader>
              }
            />
          ))}
          <Route path="*" element={<NotFound />} />
        </Routes>
      </main>
      {page && helpPath === path ? (
        <HelpDrawer title={page.title} content={page.help} onClose={() => setHelpPath(null)} />
      ) : null}
    </div>
  );
}

/**
 * PageWithHeader renders the header strip every routed page shares, from
 * the registry entry rather than from the page body. The strip is its own
 * band above the scrolling content, which is why trellis wraps the header
 * rather than stacking it with the page like the siblings do.
 */
function PageWithHeader({
  page,
  children,
  onHelp,
}: {
  page: PageConfig;
  children: ReactNode;
  onHelp: () => void;
}) {
  return (
    <>
      <div className="border-b border-hairline px-6 pt-6">
        <PageHeader
          icon={page.icon}
          eyebrow={page.eyebrow}
          title={page.title}
          description={page.description}
          onHelp={onHelp}
        />
      </div>
      <Suspense fallback={<PageLoading />}>{children}</Suspense>
    </>
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
 * Says what it is rather than pretending. A blank pane reads as a failure; this
 * reads as a plan.
 */
function NotFound() {
  const { t } = useTranslation('common');

  return (
    <div className="flex flex-1 items-center justify-center p-8">
      <div className="panel max-w-md p-6 text-center" data-testid="not-found">
        <p className="kicker">{t('emptyState.notFoundTitle')}</p>
        <p className="mt-2 text-sm text-text-secondary">{t('emptyState.notFoundBody')}</p>
        <Link
          to="/"
          className="mt-4 inline-block rounded border border-hairline px-3 py-2 text-sm text-text-primary hover:bg-surface-raised"
        >
          {t('emptyState.notFoundHome')}
        </Link>
      </div>
    </div>
  );
}
