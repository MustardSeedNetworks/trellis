// jest-dom matchers (toBeInTheDocument and friends) for the component tests.
import '@testing-library/jest-dom/vitest';
// Initialise i18next for the suite. Without it t() returns the bare key, so a
// test asserting on copy would silently assert on its absence — the failure
// mode that left niac's 492 tests unable to notice a wrecked locale file.
import '@/i18n';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';
import { installMatchMediaStub } from '@/test/matchMedia';
import { installResizeObserverStub } from '@/test/resizeObserver';

// jsdom has no matchMedia; useTheme asks it for the OS colour scheme on mount.
installMatchMediaStub();

// Nor a ResizeObserver, which the shared tables, the modal body and cmdk construct.
installResizeObserverStub();

// This project runs vitest with globals: false, so testing-library's automatic
// cleanup never registers itself — it hooks a global afterEach that does not
// exist here. Without this, renders accumulate across tests in a file and
// queries fail with "found multiple elements" for markup that is correct.
afterEach(cleanup);
