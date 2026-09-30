import { cpuPct, diskPct, memPct } from '@/lib/server'
import type { ServerView } from '@/lib/types'

export type Filter = { kind: 'all' } | { kind: 'group'; value: string } | { kind: 'country'; value: string } | { kind: 'offline' }

export interface ChipCount {
  value: string
  count: number
}

function toList(m: Map<string, number>): ChipCount[] {
  return [...m.entries()].map(([value, count]) => ({ value, count })).sort((a, b) => b.count - a.count || a.value.localeCompare(b.value))
}

/** buildChips 统计筛选标签：分组、地区（空值不计）与离线数量 */
export function buildChips(servers: ServerView[]) {
  const groups = new Map<string, number>()
  const countries = new Map<string, number>()
  let offline = 0
  for (const s of servers) {
    if (s.group) groups.set(s.group, (groups.get(s.group) ?? 0) + 1)
    if (s.country) countries.set(s.country, (countries.get(s.country) ?? 0) + 1)
    if (!s.online) offline++
  }
  return { groups: toList(groups), countries: toList(countries), offline }
}

export function applyFilter(servers: ServerView[], filter: Filter, query: string): ServerView[] {
  const q = query.trim().toLowerCase()
  return servers.filter((s) => {
    if (filter.kind === 'group' && s.group !== filter.value) return false
    if (filter.kind === 'country' && s.country !== filter.value) return false
    if (filter.kind === 'offline' && s.online) return false
    return !q || s.name.toLowerCase().includes(q)
  })
}

export function sameFilter(a: Filter, b: Filter): boolean {
  if (a.kind !== b.kind) return false
  if ((a.kind === 'group' || a.kind === 'country') && (b.kind === 'group' || b.kind === 'country')) return a.value === b.value
  return true
}

export type SortKey = 'name' | 'cpu' | 'mem' | 'disk' | 'traffic' | 'speed' | 'uptime' | 'expire' | 'availability'

const MISSING = -1
const NEVER = Number.MAX_SAFE_INTEGER

function sortValue(s: ServerView, key: SortKey): number | string {
  const m = s.metrics
  switch (key) {
    case 'name':
      return s.name
    case 'cpu':
      return m ? cpuPct(s) : MISSING
    case 'mem':
      return m ? memPct(s) : MISSING
    case 'disk':
      return m ? diskPct(s) : MISSING
    case 'traffic':
      return s.traffic.used
    case 'speed':
      return m && s.online ? m.net_in_speed + m.net_out_speed : MISSING
    case 'uptime':
      return m ? m.uptime : MISSING
    case 'expire':
      return s.expire_at ?? NEVER
    case 'availability':
      return s.uptime_24h ?? MISSING
  }
}

/** sortBy 列表视图表头排序，返回新数组 */
export function sortBy(list: ServerView[], key: SortKey, desc: boolean): ServerView[] {
  return [...list].sort((a, b) => {
    const va = sortValue(a, key)
    const vb = sortValue(b, key)
    const r = typeof va === 'string' && typeof vb === 'string' ? va.localeCompare(vb) : Number(va) - Number(vb)
    return desc ? -r : r
  })
}
