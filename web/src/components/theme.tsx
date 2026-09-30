import { useEffect, useMemo, useState, type ReactNode } from 'react'

import { ThemeContext, type Theme, type ThemeState } from './theme-context'

const KEY = 'sss.theme'
const QUERY = '(prefers-color-scheme: dark)'

function readTheme(): Theme {
  const v = localStorage.getItem(KEY)
  return v === 'light' || v === 'dark' ? v : 'system'
}

/** ThemeProvider 管理亮/暗/跟随系统主题，并同步到 <html> 的 dark 类 */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(readTheme)
  const [systemDark, setSystemDark] = useState(() => window.matchMedia(QUERY).matches)

  useEffect(() => {
    const mq = window.matchMedia(QUERY)
    const onChange = () => setSystemDark(mq.matches)
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [])

  const resolved: 'light' | 'dark' = theme === 'system' ? (systemDark ? 'dark' : 'light') : theme

  useEffect(() => {
    document.documentElement.classList.toggle('dark', resolved === 'dark')
  }, [resolved])

  const value = useMemo<ThemeState>(
    () => ({
      theme,
      resolved,
      setTheme: (t) => {
        localStorage.setItem(KEY, t)
        setThemeState(t)
      },
    }),
    [theme, resolved],
  )
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
}
