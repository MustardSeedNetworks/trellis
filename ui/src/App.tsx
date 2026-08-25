import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Route, Routes } from 'react-router';
import { type PageConfig, usePages } from '@/pageRegistry';
import { PageHeader } from '@/ui/PageHeader';
import { Sidebar } from '@/ui/Sidebar';

/**
 * Shell + routes. The rail, page header and status rollup are the family shell
 * (see ui/SHELL.md in the sibling products); everything inside a route is
 * trellis's own.
 *
 * Surveys, Import and Coverage have pages. The remaining nav items are listed
 * in navGroups.ts and land on the placeholder below until they are built,
 * which is deliberate: an item that routes somewhere honest is easier to
 * review than a rail that hides how much is left.
 */
export function App() {
  const pages = usePages();

  return (
    <div className="flex h-screen bg-surface-base text-text-primary">
      <Sidebar version={__APP_VERSION__} />
      <main className="flex flex-1 flex-col overflow-hidden">
        <Routes>
          {pages.map((page) => (
            <Route
              key={page.path}
              path={page.path}
              element={
                <PageWithHeader page={page}>
                  <page.component />
                </PageWithHeader>
              }
            />
          ))}
          <Route path="*" element={<NotBuiltYet />} />
        </Routes>
      </main>
    </div>
  );
}

/**
 * PageWithHeader renders the header strip every routed page shares, from
 * the registry entry rather than from the page body. The strip is its own
 * band above the scrolling content, which is why trellis wraps the header
 * rather than stacking it with the page like the siblings do.
 */
function PageWithHeader({ page, children }: { page: PageConfig; children: ReactNode }) {
  return (
    <>
      <div className="border-b border-hairline px-6 pt-6">
        <PageHeader
          icon={page.icon}
          eyebrow={page.eyebrow}
          title={page.title}
          description={page.description}
        />
      </div>
      {children}
    </>
  );
}

/**
 * Says what it is rather than pretending. A blank pane reads as a failure; this
 * reads as a plan.
 */
function NotBuiltYet() {
  const { t } = useTranslation('common');

  return (
    <div className="flex flex-1 items-center justify-center p-8">
      <div className="panel max-w-md p-6 text-center">
        <p className="kicker">{t('emptyState.notBuiltTitle')}</p>
        <p className="mt-2 text-sm text-text-secondary">{t('emptyState.notBuiltBody')}</p>
      </div>
    </div>
  );
}
