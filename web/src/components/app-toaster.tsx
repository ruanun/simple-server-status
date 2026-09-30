import { Toaster } from 'sonner'

import { useTheme } from './theme-context'

export function AppToaster() {
  const { resolved } = useTheme()
  return <Toaster theme={resolved} position="top-center" />
}
