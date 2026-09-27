import { Navigate, Outlet, useLocation } from 'react-router-dom'

import { useToken } from '@/lib/auth'

/** RequireAuth 未登录时跳转登录页，并记录来源路径以便登录后返回 */
export function RequireAuth() {
  const token = useToken()
  const location = useLocation()
  if (!token) return <Navigate to="/login" replace state={{ from: location.pathname }} />
  return <Outlet />
}
