import { Route, Routes } from 'react-router';
import { SurveysPage } from '@/pages/SurveysPage';
import { Sidebar } from '@/ui/Sidebar';

/**
 * Shell + routes. The rail, page header and status rollup are the family shell
 * (see ui/SHELL.md in the sibling products); everything inside a route is
 * trellis's own.
 *
 * Only Surveys has a page so far. The remaining nav items are listed in
 * navGroups.ts and land on the placeholder below until their pages are built,
 * which is deliberate: an item that routes somewhere honest is easier to
 * review than a rail that hides how much is left.
 */
export function App() {
  return (
    <div className="flex h-screen bg-surface-base text-text-primary">
      <Sidebar version={__APP_VERSION__} />
      <main className="flex flex-1 flex-col overflow-hidden">
        <Routes>
          <Route path="/" element={<SurveysPage />} />
          <Route path="*" element={<NotBuiltYet />} />
        </Routes>
      </main>
    </div>
  );
}

/**
 * Says what it is rather than pretending. A blank pane reads as a failure; this
 * reads as a plan.
 */
function NotBuiltYet() {
  return (
    <div className="flex flex-1 items-center justify-center p-8">
      <div className="panel max-w-md p-6 text-center">
        <p className="kicker">Not built yet</p>
        <p className="mt-2 text-sm text-text-secondary">
          This section is listed in the rail so the shape of the product is visible, but its page
          has not been built.
        </p>
      </div>
    </div>
  );
}
