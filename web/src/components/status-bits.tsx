import { useTranslation } from 'react-i18next'

import { useLang } from '@/i18n/use-lang'
import { formatDuration } from '@/lib/format'
import { expiryLevel } from '@/lib/server'
import type { ServerView } from '@/lib/types'
import { cn } from '@/lib/utils'

export function OnlineDot({ online }: { online: boolean }) {
  return <span className={cn('inline-block size-1.5 shrink-0 rounded-full', online ? 'bg-online' : 'bg-bad')} />
}

/** StatusPill 在线显示运行时长，离线显示红色标记 */
export function StatusPill({ server }: { server: ServerView }) {
  const { t } = useTranslation()
  const lang = useLang()
  if (!server.online) {
    return (
      <span className="inline-flex shrink-0 items-center gap-1 rounded-full border border-bad/30 px-2 py-0.5 text-[11px] text-bad">
        <OnlineDot online={false} />
        {t('status.offline')}
      </span>
    )
  }
  return (
    <span className="inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-0.5 text-[11px]">
      <OnlineDot online />
      {server.metrics ? t('status.onlineFor', { duration: formatDuration(server.metrics.uptime, lang) }) : t('status.online')}
    </span>
  )
}

/** ExpiryText 到期提示：长期 / N 天后到期（≤7 天琥珀）/ 已过期（红） */
export function ExpiryText({ days }: { days: number | null }) {
  const { t } = useTranslation()
  if (days == null) return <span>{t('expire.never')}</span>
  const level = expiryLevel(days)
  return (
    <span className={cn(level === 'warn' && 'text-warn', level === 'bad' && 'text-bad')}>
      {days <= 0 ? t('expire.expired') : t('expire.days', { count: days })}
    </span>
  )
}
