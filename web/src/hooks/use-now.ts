import { useEffect, useState } from 'react'

/** useNow 当前 Unix 秒，按间隔刷新（用于“x 秒前”“x 天后到期”） */
export function useNow(intervalMs = 1000): number {
  const [now, setNow] = useState(() => Math.floor(Date.now() / 1000))
  useEffect(() => {
    const id = setInterval(() => setNow(Math.floor(Date.now() / 1000)), intervalMs)
    return () => clearInterval(id)
  }, [intervalMs])
  return now
}
