import { api } from './api'
import type { AdminServer, NotifySettings, NotifyTestResult, Overview, ServerInput, Settings } from './types'

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

/** adminApi 管理接口（均需登录） */
export const adminApi = {
  me: () => api.get<{ username: string }>('/api/auth/me'),
  login: (username: string, password: string) => api.post<{ token: string; username: string }>('/api/auth/login', { username, password }),
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
  saveSettings: (s: Settings) => api.put<Settings>('/api/admin/settings', s),
  testNotify: (cfg: NotifySettings) => api.post<NotifyTestResult>('/api/admin/notify/test', cfg),
  exportData: () => api.get<unknown>('/api/admin/export'),
  importData: (file: unknown) => api.post<{ servers: number }>('/api/admin/import', file),
  overview: () => api.get<Overview>('/api/admin/overview'),
}
