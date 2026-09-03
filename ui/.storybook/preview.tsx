import type { Preview } from '@storybook/react-vite';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { I18nextProvider } from 'react-i18next';
import { MemoryRouter } from 'react-router';
import i18n from '../src/i18n';
import '../src/index.css';

const preview: Preview = {
  parameters: {
    a11y: {
      // Blocking from the first story. 'todo' surfaces violations in the
      // Storybook UI and cannot fail a build; stem ran that way for months
      // and an accessibility regression anywhere passed (stem#931).
      test: 'error',
    },
  },

  decorators: [
    /* Every component reads translations, most read a query client, and the
       survey detail renders router links. A story that throws still
       typechecks, lints and builds, so the providers are here rather than
       repeated per story where one would be forgotten. */
    (Story) => (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider
          client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
        >
          <MemoryRouter>
            <div className="bg-surface-base p-6 text-text-primary">
              <Story />
            </div>
          </MemoryRouter>
        </QueryClientProvider>
      </I18nextProvider>
    ),
  ],
};

export default preview;
