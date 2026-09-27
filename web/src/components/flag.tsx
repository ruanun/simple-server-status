import { cn } from '@/lib/utils'

// 只收录 flag-icons 的 4x3 国旗，按需作为独立资源加载（不内联进 JS/CSS）
const urls = import.meta.glob<string>('/node_modules/flag-icons/flags/4x3/*.svg', { query: '?url', import: 'default', eager: true })

// 键形如 /node_modules/flag-icons/flags/4x3/hk.svg，转换为小写国家代码
const FLAGS = new Map(Object.entries(urls).map(([path, url]) => [path.slice(path.lastIndexOf('/') + 1, -'.svg'.length), url]))

/** flagUrl 国家代码对应的国旗地址，不存在时返回 undefined */
function flagUrl(code: string): string | undefined {
  return /^[A-Za-z]{2}$/.test(code) ? FLAGS.get(code.toLowerCase()) : undefined
}

/** Flag 国旗图标（flag-icons 4x3 SVG），代码无效或无对应国旗时不渲染 */
export function Flag({ code, className }: { code: string; className?: string }) {
  const src = flagUrl(code)
  if (!src) return null
  const upper = code.toUpperCase()
  return (
    <img
      src={src}
      alt={upper}
      title={upper}
      aria-label={upper}
      loading="lazy"
      decoding="async"
      className={cn('inline-block h-[1em] w-[1.333em] shrink-0 rounded-[2px] object-cover', className)}
    />
  )
}
