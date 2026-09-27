import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'

import { Button } from '@/components/ui/button'

export function NotFound() {
  const { t } = useTranslation()
  return (
    <main className="flex min-h-svh flex-col items-center justify-center gap-4 px-4">
      <p className="text-sm text-muted-foreground">{t('notFound.title')}</p>
      <Button asChild variant="outline" size="sm">
        <Link to="/">{t('notFound.back')}</Link>
      </Button>
    </main>
  )
}
