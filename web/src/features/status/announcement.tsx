import { Megaphone } from 'lucide-react'
import type { ReactNode } from 'react'

// 只匹配 URL 合法的 ASCII 字符，避免吞掉紧随其后的中文标点
const URL_RE = /(https?:\/\/[\w\-.~:/?#[\]@!$&'()*+,;=%]+)/g
// 链接末尾的这些标点通常属于句子而不是链接
const TRAILING_RE = /[.,;:!?)']+$/

/** Announcement 站点公告：保留换行，网址转为链接，其余内容按纯文本渲染 */
export function Announcement({ text }: { text: string }) {
  const parts: ReactNode[] = text.split(URL_RE).flatMap((part, i) => {
    if (i % 2 === 0) return [part]
    const tail = TRAILING_RE.exec(part)?.[0] ?? ''
    const url = part.slice(0, part.length - tail.length)
    return [
      <a key={i} href={url} target="_blank" rel="noopener noreferrer" className="underline underline-offset-2 hover:text-foreground">
        {url}
      </a>,
      tail,
    ]
  })
  return (
    <div role="note" className="flex gap-2 rounded-lg border bg-card px-4 py-3 text-sm text-muted-foreground">
      <Megaphone className="mt-0.5 size-4 shrink-0" aria-hidden />
      <p className="min-w-0 whitespace-pre-line break-words">{parts}</p>
    </div>
  )
}
