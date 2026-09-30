import type { SortKey } from './filters'

export type SortState = { key: SortKey; desc: boolean } | null

const KEY = 'sss.sort'
const KEYS: SortKey[] = ['name', 'cpu', 'mem', 'disk', 'traffic', 'speed', 'uptime', 'expire', 'availability']

/** loadSort 读取保存的排序；无效或未设置时为 null（默认顺序） */
export function loadSort(): SortState {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) ?? 'null') as SortState
    return v && KEYS.includes(v.key) && typeof v.desc === 'boolean' ? v : null
  } catch {
    return null
  }
}

export function saveSort(s: SortState) {
  if (s) localStorage.setItem(KEY, JSON.stringify(s))
  else localStorage.removeItem(KEY)
}

/** defaultDesc 名称默认升序，其余默认降序 */
export function defaultDesc(key: SortKey): boolean {
  return key !== 'name'
}

export const SORT_KEYS = KEYS
