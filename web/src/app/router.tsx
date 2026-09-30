import { createBrowserRouter, Navigate } from 'react-router-dom'

import { RequireAuth } from '@/features/auth/require-auth'
import { StatusPage } from '@/features/status/status-page'

import { NotFound } from './not-found'
import { RouteError } from './route-error'

export const router = createBrowserRouter([
  {
    errorElement: <RouteError />,
    children: [
      { path: '/', element: <StatusPage /> },
      { path: '/server/:id', lazy: async () => ({ Component: (await import('@/features/detail/detail-page')).DetailPage }) },
      { path: '/login', lazy: async () => ({ Component: (await import('@/features/auth/login-page')).LoginPage }) },
      {
        path: '/admin',
        element: <RequireAuth />,
        children: [
          {
            lazy: async () => ({ Component: (await import('@/features/admin/admin-layout')).AdminLayout }),
            children: [
              { index: true, element: <Navigate to="servers" replace /> },
              { path: 'servers', lazy: async () => ({ Component: (await import('@/features/admin/servers-page')).ServersPage }) },
              { path: 'settings', lazy: async () => ({ Component: (await import('@/features/admin/settings-page')).SettingsPage }) },
              { path: 'events', lazy: async () => ({ Component: (await import('@/features/admin/events-page')).EventsPage }) },
            ],
          },
        ],
      },
      { path: '*', element: <NotFound /> },
    ],
  },
])
