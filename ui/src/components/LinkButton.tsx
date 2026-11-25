import type { ReactNode } from 'react';
import { Link, type To } from 'react-router';
import { cx } from '@/shared/cx';

/**
 * LinkButton: navigation that looks like the shared `Button`.
 *
 * The shared Button renders a `<button>`, and going to another page is a link
 * (it opens in a new tab, shows its address, and a screen reader announces it
 * as one). The shared set has no link form, so this carries the two faces
 * trellis needs, taken from Button's own tokens. Retire it when the shared
 * Button gains one.
 */
const FACES = {
  primary: 'border-transparent bg-brand-primary text-on-brand hover:bg-brand-primary/90',
  secondary: 'border-surface-border bg-surface-raised text-text-primary hover:bg-surface-hover',
} as const;

export function LinkButton({
  to,
  variant = 'secondary',
  children,
  'data-testid': testId,
}: {
  to: To;
  variant?: keyof typeof FACES;
  children: ReactNode;
  'data-testid'?: string;
}) {
  return (
    <Link
      to={to}
      data-variant={variant}
      data-testid={testId}
      className={cx(
        'inline-flex min-h-10 items-center justify-center gap-2 rounded-lg border px-4 text-sm font-medium transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-primary',
        FACES[variant],
      )}
    >
      {children}
    </Link>
  );
}
