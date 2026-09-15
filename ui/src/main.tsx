import '@fontsource-variable/manrope';
import '@fontsource-variable/jetbrains-mono';
import { QueryClientProvider } from '@tanstack/react-query';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router';
import { App } from '@/App';
import { applyStoredTheme } from '@/hooks/useTheme';
import { createQueryClient } from '@/lib/queryClient';
import { AuthGate } from '@/ui/AuthGate';
// Side-effect import: initialises i18next before any component renders.
import '@/i18n';
import './index.css';

// Before the first paint, not in an effect: AuthGate's login screen renders
// above App, and an effect would flash the light palette on a dark desktop.
applyStoredTheme();

const queryClient = createQueryClient();

const container = document.getElementById('root');
if (!container) {
  throw new Error('root element not found');
}

createRoot(container).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthGate>
          <App />
        </AuthGate>
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
);
