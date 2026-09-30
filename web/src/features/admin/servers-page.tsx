import { closestCenter, DndContext, KeyboardSensor, PointerSensor, useSensor, useSensors, type DragEndEvent } from '@dnd-kit/core'
import { arrayMove, SortableContext, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { BellOff, EyeOff, GripVertical, MoreHorizontal, Plus, StickyNote } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Flag } from '@/components/flag'
import { AvailabilityText, LastReport, OnlineDot } from '@/components/status-bits'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useNow } from '@/hooks/use-now'
import { useLang } from '@/i18n/use-lang'
import { adminApi, adminKeys } from '@/lib/admin-api'
import { errorMessage } from '@/lib/api'
import type { Lang } from '@/lib/format'
import { formatDate } from '@/lib/format'
import type { AdminServer } from '@/lib/types'
import { cn } from '@/lib/utils'

import { InstallDialog } from './install-dialog'
import { OverviewBar } from './overview-bar'
import { ServerFormDialog } from './server-form'

interface RowProps {
  server: AdminServer
  lang: Lang
  now: number
  onEdit: () => void
  onInstall: () => void
  onUpgrade: () => void
  onReset: () => void
  onDelete: () => void
}

function SortableRow({ server: s, lang, now, onEdit, onInstall, onUpgrade, onReset, onDelete }: RowProps) {
  const { t } = useTranslation()
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: s.id })
  return (
    <TableRow ref={setNodeRef} style={{ transform: CSS.Transform.toString(transform), transition }} className={cn(isDragging && 'relative z-10 bg-accent')}>
      <TableCell className="w-8">
        <button type="button" className="cursor-grab touch-none text-muted-foreground" aria-label={t('admin.drag')} {...attributes} {...listeners}>
          <GripVertical className="size-4" />
        </button>
      </TableCell>
      <TableCell>
        <div className="flex min-w-0 items-center gap-2">
          <OnlineDot online={s.online} />
          <span className="truncate font-medium">{s.name}</span>
          <Flag code={s.country} />
          {s.hidden && (
            <span title={t('form.hidden')} className="inline-flex text-muted-foreground">
              <EyeOff className="size-3.5" aria-hidden />
              <span className="sr-only">{t('form.hidden')}</span>
            </span>
          )}
          {s.note && (
            <span title={s.note} className="inline-flex text-muted-foreground">
              <StickyNote className="size-3.5" aria-hidden />
              <span className="sr-only">
                {t('admin.note')}：{s.note}
              </span>
            </span>
          )}
          {s.notify_muted && (
            <span title={t('form.notifyMuted')} className="inline-flex text-muted-foreground">
              <BellOff className="size-3.5" aria-hidden />
              <span className="sr-only">{t('form.notifyMuted')}</span>
            </span>
          )}
        </div>
      </TableCell>
      <TableCell className="hidden sm:table-cell">{s.group || '—'}</TableCell>
      <TableCell className="hidden font-mono text-xs md:table-cell">{s.last_ip || '—'}</TableCell>
      <TableCell className="hidden text-xs tabular md:table-cell">
        <LastReport ts={s.last_seen} now={now} />
      </TableCell>
      <TableCell className="hidden text-xs md:table-cell">{s.expire_at ? formatDate(s.expire_at, lang) : t('expire.never')}</TableCell>
      <TableCell className="hidden text-xs tabular lg:table-cell">{s.report_interval}s</TableCell>
      <TableCell className="hidden text-xs tabular lg:table-cell">
        {s.agent_version ? (
          <span className={cn(s.outdated && 'text-warn')} title={s.outdated ? t('admin.outdated') : undefined}>
            {s.agent_version}
          </span>
        ) : (
          '—'
        )}
      </TableCell>
      <TableCell className="hidden text-xs lg:table-cell">{s.uptime_24h == null ? '—' : <AvailabilityText value={s.uptime_24h} />}</TableCell>
      <TableCell className="w-10 text-right">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="size-8" aria-label={t('admin.actions')}>
              <MoreHorizontal className="size-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={onEdit}>{t('admin.edit')}</DropdownMenuItem>
            <DropdownMenuItem onSelect={onInstall}>{t('admin.install')}</DropdownMenuItem>
            {s.outdated && <DropdownMenuItem onSelect={onUpgrade}>{t('install.upgradeTitle')}</DropdownMenuItem>}
            <DropdownMenuItem onSelect={onReset}>{t('admin.resetSecret')}</DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={onDelete} className="text-bad focus:text-bad">
              {t('admin.delete')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </TableCell>
    </TableRow>
  )
}

type Confirm = { kind: 'delete' | 'reset'; server: AdminServer }

export function ServersPage() {
  const { t } = useTranslation()
  const lang = useLang()
  const now = useNow()
  const qc = useQueryClient()
  const { data, isLoading, error } = useQuery({ queryKey: adminKeys.servers, queryFn: adminApi.servers, refetchInterval: 10_000 })
  const [editing, setEditing] = useState<AdminServer | 'new' | null>(null)
  const [installFor, setInstallFor] = useState<{ server: AdminServer; mode: 'install' | 'upgrade' } | null>(null)
  const [confirm, setConfirm] = useState<Confirm | null>(null)
  const [order, setOrder] = useState<string[] | null>(null)

  // 拖拽后先按本地顺序显示，保存完成并刷新列表后再使用服务端顺序
  const servers = useMemo(() => {
    const list = data ?? []
    if (!order) return list
    const byId = new Map(list.map((s) => [s.id, s]))
    return order.map((id) => byId.get(id)).filter((s): s is AdminServer => s !== undefined)
  }, [data, order])

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  const onDragEnd = ({ active, over }: DragEndEvent) => {
    if (!over || active.id === over.id) return
    const ids = servers.map((s) => s.id)
    const next = arrayMove(ids, ids.indexOf(String(active.id)), ids.indexOf(String(over.id)))
    setOrder(next)
    adminApi
      .order(next)
      .then(() => qc.invalidateQueries({ queryKey: adminKeys.servers }))
      .catch((err: unknown) => toast.error(errorMessage(err)))
      .finally(() => setOrder(null))
  }

  const runConfirm = async () => {
    if (!confirm) return
    const { kind, server } = confirm
    setConfirm(null)
    try {
      if (kind === 'delete') {
        await adminApi.remove(server.id)
        toast.success(t('admin.deleted'))
      } else {
        await adminApi.resetSecret(server.id)
        toast.success(t('admin.secretReset'))
        setInstallFor({ server, mode: 'install' })
      }
      await qc.invalidateQueries({ queryKey: adminKeys.servers })
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  let body
  if (isLoading) body = <Skeleton className="h-40 rounded-lg" />
  else if (error) body = <p className="text-sm text-bad">{errorMessage(error)}</p>
  else if (servers.length === 0) body = <p className="rounded-lg border bg-card py-16 text-center text-sm text-muted-foreground">{t('admin.empty')}</p>
  else
    body = (
      <div className="overflow-x-auto rounded-lg border bg-card">
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
          <SortableContext items={servers.map((s) => s.id)} strategy={verticalListSortingStrategy}>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-8" />
                  <TableHead>{t('admin.name')}</TableHead>
                  <TableHead className="hidden sm:table-cell">{t('admin.group')}</TableHead>
                  <TableHead className="hidden md:table-cell">{t('admin.ip')}</TableHead>
                  <TableHead className="hidden md:table-cell">{t('admin.lastSeen')}</TableHead>
                  <TableHead className="hidden md:table-cell">{t('admin.expire')}</TableHead>
                  <TableHead className="hidden lg:table-cell">{t('admin.interval')}</TableHead>
                  <TableHead className="hidden lg:table-cell">{t('admin.agent')}</TableHead>
                  <TableHead className="hidden lg:table-cell">{t('admin.availability')}</TableHead>
                  <TableHead className="w-10">
                    <span className="sr-only">{t('admin.actions')}</span>
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {servers.map((s) => (
                  <SortableRow
                    key={s.id}
                    server={s}
                    lang={lang}
                    now={now}
                    onEdit={() => setEditing(s)}
                    onInstall={() => setInstallFor({ server: s, mode: 'install' })}
                    onUpgrade={() => setInstallFor({ server: s, mode: 'upgrade' })}
                    onReset={() => setConfirm({ kind: 'reset', server: s })}
                    onDelete={() => setConfirm({ kind: 'delete', server: s })}
                  />
                ))}
              </TableBody>
            </Table>
          </SortableContext>
        </DndContext>
      </div>
    )

  return (
    <div className="mx-auto max-w-6xl space-y-4">
      <div className="flex items-center justify-between gap-2">
        <h1 className="text-lg font-semibold">{t('admin.title')}</h1>
        <Button onClick={() => setEditing('new')}>
          <Plus className="size-4" />
          {t('admin.add')}
        </Button>
      </div>
      <OverviewBar />
      {body}

      {editing !== null && (
        <ServerFormDialog
          key={editing === 'new' ? 'new' : editing.id}
          server={editing === 'new' ? null : editing}
          onClose={() => setEditing(null)}
          onSaved={(saved, created) => {
            setEditing(null)
            if (created) setInstallFor({ server: saved, mode: 'install' })
          }}
        />
      )}
      <InstallDialog server={installFor?.server ?? null} mode={installFor?.mode} onClose={() => setInstallFor(null)} />
      <AlertDialog open={confirm !== null} onOpenChange={(o) => !o && setConfirm(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{confirm && t(confirm.kind === 'delete' ? 'admin.deleteTitle' : 'admin.resetTitle', { name: confirm.server.name })}</AlertDialogTitle>
            <AlertDialogDescription>{confirm && t(confirm.kind === 'delete' ? 'admin.deleteDesc' : 'admin.resetDesc')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void runConfirm()} className={confirm?.kind === 'delete' ? 'bg-destructive text-white hover:bg-destructive/90' : undefined}>
              {t('common.confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
