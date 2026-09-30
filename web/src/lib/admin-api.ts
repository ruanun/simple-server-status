import { api } from './api'
import type {
  AdminOutage,
  AdminServer,
  CaptchaChallenge,
  LoginInput,
  NotifyLogItem,
  NotifySettings,
  NotifyTestResult,
  Overview,
  Paged,
  ServerInput,
  Settings,
} from './types'

export const adminKeys = {
  servers: ['admin', 'servers'] as const,
  settings: ['admin', 'settings'] as const,
  me: ['admin', 'me'] as const,
  overview: ['admin', 'overview'] as const,
}

export interface InstallCommands {
  linux: string
  windows: string
}

const enc = encodeURIComponent

const qs = (o: Record<string, string | number | undefined>) =>
  Object.entries(o)
    .filter(([, v]) => v !== undefined && v !== '')
    .map(([k, v]) => `${k}=${enc(String(v))}`)
    .join('&')

/** adminApi 管理接口（均需登录） */
export const adminApi = {
  me: () => api.get<{ username: string; version: string }>('/api/auth/me'),
  login: (input: LoginInput) => api.post<{ token: string; username: string }>('/api/auth/login', input),
  captcha: () => api.get<CaptchaChallenge>('/api/auth/captcha'),
  changePassword: (oldPassword: string, newPassword: string) =>
    api.put<{ token: string }>('/api/auth/password', { old_password: oldPassword, new_password: newPassword }),
  servers: () => api.get<AdminServer[]>('/api/admin/servers'),
  create: (input: ServerInput) => api.post<AdminServer>('/api/admin/servers', input),
  update: (id: string, input: ServerInput) => api.put<AdminServer>(`/api/admin/servers/${enc(id)}`, input),
  remove: (id: string) => api.del<unknown>(`/api/admin/servers/${enc(id)}`),
  resetSecret: (id: string) => api.post<{ secret: string }>(`/api/admin/servers/${enc(id)}/reset-secret`),
  install: (id: string, dashboard: string) => api.get<InstallCommands>(`/api/admin/servers/${enc(id)}/install?dashboard=${enc(dashboard)}`),
  order: (ids: string[]) => api.put<unknown>('/api/admin/server-order', { ids }),
  settings: () => api.get<Settings>('/api/admin/settings'),
  // turnstile_token：新启用 Turnstile 或修改其密钥时，需附带一个用新密钥通过验证的 token
  saveSettings: (s: Settings & { turnstile_token?: string }) => api.put<Settings>('/api/admin/settings', s),
  testNotify: (cfg: NotifySettings) => api.post<NotifyTestResult>('/api/admin/notify/test', cfg),
  exportData: () => api.get<unknown>('/api/admin/export'),
  // captcha_kept：文件中的 Turnstile 配置未经核验，保留了当前的验证码设置
  importData: (file: unknown) => api.post<{ servers: number; captcha_kept?: boolean }>('/api/admin/import', file),
  overview: () => api.get<Overview>('/api/admin/overview'),
  outages: (q: { server?: string; page: number }) =>
    api.get<Paged<AdminOutage>>(`/api/admin/outages?${qs({ server_id: q.server, page: q.page, size: 50 })}`),
  notifyLog: (q: { server?: string; status?: string; page: number }) =>
    api.get<Paged<NotifyLogItem>>(`/api/admin/notify-log?${qs({ server_id: q.server, status: q.status, page: q.page, size: 50 })}`),
}
