import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, LogOut, Menu, Server, Settings } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, NavLink, Outlet } from 'react-router-dom'

import { LangToggle, ThemeToggle } from '@/components/header-controls'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { useSite } from '@/features/site/use-site'
import { adminApi, adminKeys } from '@/lib/admin-api'
import { tokenStore } from '@/lib/api'
import { cn } from '@/lib/utils'

/** AdminLayout 后台布局：桌面左侧窄侧栏，移动端抽屉 */
export function AdminLayout() {
  const { t } = useTranslation()
  const site = useSite()
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  // 进入后台即校验 token；失效时接口返回 401，登录态被清除，路由守卫自动跳转登录页
  const me = useQuery({ queryKey: adminKeys.me, queryFn: adminApi.me })

  const links = [
    { to: '/admin/servers', icon: Server, label: t('nav.servers') },
    { to: '/admin/settings', icon: Settings, label: t('nav.settings') },
  ]
  // 退出登录：清除 token 并丢弃后台数据与登录态的服务器列表缓存（含隐藏服务器）
  const logout = () => {
    tokenStore.clear()
    qc.removeQueries({ queryKey: ['admin'] })
    qc.removeQueries({ queryKey: ['servers', 'authed'] })
  }
  const item = 'flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm'

  const sidebar = (
    <>
      <nav className="flex flex-col gap-1">
        {links.map((l) => (
          <NavLink
            key={l.to}
            to={l.to}
            onClick={() => setOpen(false)}
            className={({ isActive }) => cn(item, isActive ? 'bg-accent font-medium' : 'text-muted-foreground hover:bg-accent/60')}
          >
            <l.icon className="size-4" />
            {l.label}
          </NavLink>
        ))}
      </nav>
      <div className="mt-auto flex flex-col gap-1">
        <Link to="/" className={cn(item, 'text-muted-foreground hover:bg-accent/60')}>
          <ArrowLeft className="size-4" />
          {t('nav.status')}
        </Link>
        <button type="button" onClick={logout} className={cn(item, 'text-muted-foreground hover:bg-accent/60')}>
          <LogOut className="size-4" />
          {t('nav.logout')}
          {me.data && <span className="ml-auto truncate text-xs">{me.data.username}</span>}
        </button>
      </div>
    </>
  )

  return (
    <div className="flex min-h-svh">
      <aside className="hidden w-56 shrink-0 flex-col gap-4 border-r bg-card p-3 md:flex">
        <div className="px-3 py-2 font-semibold tracking-tight">{site.site_title}</div>
        {sidebar}
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 items-center gap-2 border-b px-4">
          <Button variant="ghost" size="icon" className="md:hidden" onClick={() => setOpen(true)} aria-label={t('nav.menu')}>
            <Menu className="size-4" />
          </Button>
          <div className="ml-auto flex items-center gap-1">
            <LangToggle />
            <ThemeToggle />
          </div>
        </header>
        <main className="min-w-0 flex-1 p-4 md:p-6">
          <Outlet />
        </main>
      </div>
      <Sheet open={open} onOpenChange={setOpen}>
        <SheetContent side="left" className="flex w-64 flex-col gap-4 p-3" aria-describedby={undefined}>
          <SheetHeader>
            <SheetTitle>{site.site_title}</SheetTitle>
          </SheetHeader>
          {sidebar}
        </SheetContent>
      </Sheet>
    </div>
  )
}
