/**
 * Components with a defaulted prop are compiled by the React Compiler (#721).
 *
 * babel-plugin-react-compiler 1.0.0 is built on @babel/types 7. Under
 * @babel/core 8 it cannot lower a defaulted destructured prop
 * (`className = ''`) and skips the whole component without failing the build.
 * A compiled component re-rendered with unchanged props reuses what it
 * derived from `t`, so a stable `t` is not called again; an uncompiled one
 * calls it on every render. Each case fails under @babel/core 8, and when the
 * compiler is removed from vitest.config.ts.
 */

import { render } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { PageHeader } from './PageHeader';
import { StatusRollup } from './StatusRollup';

const { t } = vi.hoisted(() => ({ t: vi.fn((key: string) => key) }));
vi.mock('react-i18next', () => ({ useTranslation: () => ({ t }) }));

describe('components with a defaulted prop are compiled', () => {
  it('StatusRollup does not re-translate its state labels', () => {
    t.mockClear();
    const figures = [{ label: 'Frames', value: '1.2M' }];
    const { rerender } = render(<StatusRollup state="ok" headline="Fine" figures={figures} />);
    const first = t.mock.calls.length;
    rerender(<StatusRollup state="ok" headline="Fine" figures={figures} />);
    expect(first).toBeGreaterThan(0);
    expect(t).toHaveBeenCalledTimes(first);
  });

  it('PageHeader does not re-translate its help labels', () => {
    t.mockClear();
    const onHelp = vi.fn();
    const { rerender } = render(<PageHeader title="Surveys" onHelp={onHelp} />);
    const first = t.mock.calls.length;
    rerender(<PageHeader title="Surveys" onHelp={onHelp} />);
    expect(first).toBeGreaterThan(0);
    expect(t).toHaveBeenCalledTimes(first);
  });
});
