import { createContext, useContext } from 'react'

export type Theme = 'light' | 'dark' | 'system'

export interface ThemeState {
  theme: Theme
  resolved: 'light' | 'dark'
  setTheme: (t: Theme) => void
}

export const ThemeContext = createContext<ThemeState | null>(null)

/** useTheme 读取当前主题；需在 ThemeProvider 内使用 */
export function useTheme(): ThemeState {
  const v = useContext(ThemeContext)
  if (!v) throw new Error('useTheme 必须在 ThemeProvider 内使用')
  return v
}
