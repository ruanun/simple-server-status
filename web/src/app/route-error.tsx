import { useTranslation } from 'react-i18next'
import { useRouteError } from 'react-router-dom'

import { Button } from '@/components/ui/button'
import { errorMessage } from '@/lib/api'

/** RouteError 路由级错误页：懒加载的页面代码加载失败（如面板升级后旧资源失效）时，刷新即可恢复 */
export function RouteError() {
  const { t } = useTranslation()
  const error = useRouteError()
  return (
    <main className="flex min-h-svh flex-col items-center justify-center gap-4 px-4 text-center">
      <div className="space-y-1">
        <h1 className="text-base font-semibold">{t('routeError.title')}</h1>
        <p className="text-sm text-muted-foreground">{t('routeError.desc')}</p>
      </div>
      <div className="flex gap-2">
        <Button size="sm" onClick={() => window.location.reload()}>
          {t('routeError.reload')}
        </Button>
        {/* 使用整页跳转，避免在资源失效时继续走客户端路由 */}
        <Button asChild variant="outline" size="sm">
          <a href="/">{t('routeError.home')}</a>
        </Button>
      </div>
      <pre className="max-w-lg whitespace-pre-wrap text-xs text-muted-foreground">{errorMessage(error)}</pre>
    </main>
  )
}
