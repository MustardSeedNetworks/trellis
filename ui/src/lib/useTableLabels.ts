import { useTranslation } from 'react-i18next';
import type { DataTableLabels } from '@/shared/DataTable';

/** The wording every `DataTable` in the product shares, in the operator's language. */
export function useTableLabels<T>(): DataTableLabels<T> {
  const { t } = useTranslation('common');

  return {
    selectAll: t('table.selectAll'),
    selectRow: () => t('table.selectRow'),
    actions: t('table.actions'),
    pagination: t('table.pagination'),
    previousPage: t('table.previousPage'),
    nextPage: t('table.nextPage'),
    loading: t('table.loading'),
    pageStatus: (page, pageCount, total) => t('table.pageStatus', { page, pageCount, total }),
  };
}
