import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'

import { api } from '@/lib/api'
import { useToken } from '@/lib/auth'
import { sortServers } from '@/lib/server'
import type { ServerView } from '@/lib/types'

import { RealtimeClient, type LiveMessage, type RealtimeStatus } from './client'
import { recordSpeed } from './speed-history'

/** serversKey 登录与未登录的服务器列表使用不同缓存，避免隐藏服务器残留在公开视图 */
export function serversKey(authed: boolean) {
  return ['servers', authed ? 'authed' : 'public'] as const
}

/** applyMessage 把推送消息合并进现有列表 */
export function applyMessage(prev: ServerView[] | undefined, m: LiveMessage): ServerView[] {
  if (m.type === 'snapshot') return sortServers(m.data)
  const map = new Map((prev ?? []).map((s) => [s.id, s]))
  for (const s of m.data) map.set(s.id, s)
  return sortServers([...map.values()])
}

export function wsUrl(token: string | null, loc: Pick<Location, 'protocol' | 'host'> = window.location): string {
  const scheme = loc.protocol === 'https:' ? 'wss:' : 'ws:'
  const query = token ? `?token=${encodeURIComponent(token)}` : ''
  return `${scheme}//${loc.host}/api/public/ws${query}`
}

/** useLiveServers 服务器实时列表：HTTP 快照 + WebSocket 推送，断线时每 5 秒轮询；页面隐藏时断开 */
export function useLiveServers() {
  const token = useToken()
  const authed = token !== null
  const qc = useQueryClient()
  const [status, setStatus] = useState<RealtimeStatus>('connecting')

  const query = useQuery({
    queryKey: serversKey(authed),
    queryFn: async () => {
      const list = sortServers(await api.get<ServerView[]>('/api/public/servers'))
      recordSpeed(list)
      return list
    },
  })

  useEffect(() => {
    const key = serversKey(authed)
    const client = new RealtimeClient({
      url: () => wsUrl(token),
      onMessage: (m) =>
        qc.setQueryData<ServerView[]>(key, (prev) => {
          const next = applyMessage(prev, m)
          recordSpeed(next)
          return next
        }),
      onStatus: setStatus,
      poll: () => {
        void qc.invalidateQueries({ queryKey: key })
      },
    })
    client.start()
    const onVisibility = () => {
      if (document.hidden) client.stop()
      else client.start()
    }
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      document.removeEventListener('visibilitychange', onVisibility)
      client.stop()
    }
  }, [token, authed, qc])

  return { servers: query.data ?? [], isLoading: query.isLoading, error: query.error, status }
}
