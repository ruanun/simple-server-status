import { ArrowDown, ArrowUp } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

import { defaultDesc, SORT_KEYS, type SortState } from './sort-state'

/** SortSelect 排序下拉与升降序切换；与列表视图表头共用同一排序状态 */
export function SortSelect({ sort, onSort }: { sort: SortState; onSort: (s: SortState) => void }) {
  const { t } = useTranslation()
  return (
    <div className="flex items-center gap-1">
      <Select
        value={sort?.key ?? 'default'}
        onValueChange={(v) => onSort(v === 'default' ? null : { key: v as NonNullable<SortState>['key'], desc: defaultDesc(v as NonNullable<SortState>['key']) })}
      >
        <SelectTrigger size="sm" className="w-28" aria-label={t('sort.label')}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="default">{t('sort.default')}</SelectItem>
          {SORT_KEYS.map((k) => (
            <SelectItem key={k} value={k}>
              {t(`sort.${k}`)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {sort && (
        <Button
          variant="outline"
          size="icon"
          className="size-8"
          aria-label={sort.desc ? t('sort.desc') : t('sort.asc')}
          onClick={() => onSort({ ...sort, desc: !sort.desc })}
        >
          {sort.desc ? <ArrowDown className="size-4" /> : <ArrowUp className="size-4" />}
        </Button>
      )}
    </div>
  )
}
