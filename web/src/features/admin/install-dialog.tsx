import { useQuery } from '@tanstack/react-query'
import { Copy } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { adminApi } from '@/lib/admin-api'
import { errorMessage } from '@/lib/api'
import type { AdminServer } from '@/lib/types'

function CopyBlock({ text }: { text: string }) {
  const { t } = useTranslation()
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      toast.success(t('install.copied'))
    } catch {
      toast.error(t('install.copyFailed'))
    }
  }
  return (
    <div className="relative">
      <pre className="max-h-56 overflow-auto rounded-lg bg-muted p-3 pr-12 font-mono text-xs break-all whitespace-pre-wrap">{text}</pre>
      <Button size="icon" variant="ghost" className="absolute top-1.5 right-1.5 size-7" onClick={() => void copy()} aria-label={t('install.copy')}>
        <Copy className="size-3.5" />
      </Button>
    </div>
  )
}

/** InstallDialog 显示一键安装命令（每次打开重新获取，重置密钥后即为新命令）；面板地址默认取当前页面地址，可修改；升级模式复用同一套命令 */
export function InstallDialog({ server, mode = 'install', onClose }: { server: AdminServer | null; mode?: 'install' | 'upgrade'; onClose: () => void }) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState(() => window.location.origin)
  const [dashboard, setDashboard] = useState(draft)
  const commit = () => {
    const v = draft.trim()
    if (v) setDashboard(v)
    else setDraft(dashboard)
  }
  const q = useQuery({
    queryKey: ['admin', 'install', server?.id, dashboard],
    queryFn: () => (server ? adminApi.install(server.id, dashboard) : Promise.resolve(null)),
    enabled: server !== null,
    gcTime: 0,
    staleTime: 0,
  })

  return (
    <Dialog open={server !== null} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{mode === 'upgrade' ? t('install.upgradeTitle') : t('install.title')}</DialogTitle>
          <DialogDescription>{mode === 'upgrade' ? t('install.upgradeDesc') : t('install.desc')}</DialogDescription>
        </DialogHeader>
        <div className="space-y-1.5">
          <Label htmlFor="install_dashboard">{t('install.dashboard')}</Label>
          <Input
            id="install_dashboard"
            value={draft}
            aria-describedby="install_dashboard-hint"
            onChange={(e) => setDraft(e.target.value)}
            onBlur={commit}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                commit()
              }
            }}
          />
          <p id="install_dashboard-hint" className="text-xs text-muted-foreground">
            {t('install.dashboardHint')}
          </p>
        </div>
        {q.error ? (
          <p className="text-sm text-bad">{errorMessage(q.error)}</p>
        ) : !q.data ? (
          <Skeleton className="h-24" />
        ) : (
          <Tabs defaultValue="linux">
            <TabsList>
              <TabsTrigger value="linux">Linux</TabsTrigger>
              <TabsTrigger value="windows">Windows</TabsTrigger>
            </TabsList>
            <TabsContent value="linux">
              <CopyBlock text={q.data.linux} />
            </TabsContent>
            <TabsContent value="windows">
              <CopyBlock text={q.data.windows} />
            </TabsContent>
          </Tabs>
        )}
      </DialogContent>
    </Dialog>
  )
}
