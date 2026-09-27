import { useSyncExternalStore } from 'react'

import { tokenStore } from './api'

/** useToken 当前登录 token；登录、退出或 401 失效时自动重新渲染 */
export function useToken(): string | null {
  return useSyncExternalStore(tokenStore.subscribe, tokenStore.get, () => null)
}
