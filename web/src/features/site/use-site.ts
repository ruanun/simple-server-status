import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'

import { api } from '@/lib/api'
import type { Site } from '@/lib/types'

const FALLBACK: Site = { site_title: 'Simple Server Status', show_price: false }

/** useSite 站点标题与价格可见性，并同步浏览器标题 */
export function useSite(): Site {
  const { data } = useQuery({ queryKey: ['site'], queryFn: () => api.get<Site>('/api/public/site'), staleTime: 60_000 })
  useEffect(() => {
    if (data) document.title = data.site_title
  }, [data])
  return data ?? FALLBACK
}
