// jest-dom matchers (toBeInTheDocument and friends) for the component tests.
import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

// This project runs vitest with globals: false, so testing-library's automatic
// cleanup never registers itself — it hooks a global afterEach that does not
// exist here. Without this, renders accumulate across tests in a file and
// queries fail with "found multiple elements" for markup that is correct.
afterEach(cleanup);
