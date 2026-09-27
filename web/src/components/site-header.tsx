import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'

import { Button } from '@/components/ui/button'
import { useSite } from '@/features/site/use-site'
import { useToken } from '@/lib/auth'

import { LangToggle, ThemeToggle } from './header-controls'

export function SiteHeader() {
  const site = useSite()
  const token = useToken()
  const { t } = useTranslation()
  return (
    <header className="sticky top-0 z-30 border-b bg-background/80 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-[1400px] items-center justify-between gap-2 px-4">
        <Link to="/" className="truncate font-semibold tracking-tight">
          {site.site_title}
        </Link>
        <div className="flex shrink-0 items-center gap-1">
          <LangToggle />
          <ThemeToggle />
          <Button asChild variant="ghost" size="sm">
            <Link to={token ? '/admin' : '/login'}>{token ? t('nav.admin') : t('nav.login')}</Link>
          </Button>
        </div>
      </div>
    </header>
  )
}
