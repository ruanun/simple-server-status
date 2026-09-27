import { Toaster } from 'sonner'

import { useTheme } from './theme'

export function AppToaster() {
  const { resolved } = useTheme()
  return <Toaster theme={resolved} position="top-center" />
}
