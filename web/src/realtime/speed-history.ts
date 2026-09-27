import { useSyncExternalStore } from 'react'

import type { ServerView } from '@/lib/types'

export interface SpeedSample {
  ts: number
  in: number
  out: number
}

const MAX = 300
let samples: SpeedSample[] = []
const listeners = new Set<() => void>()

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

function getSnapshot() {
  return samples
}

/** recordSpeed 记录一次全部在线服务器的总网速（最多保留 300 个样本，约 10 分钟） */
export function recordSpeed(servers: ServerView[], nowSec = Math.floor(Date.now() / 1000)): void {
  let inSum = 0
  let outSum = 0
  for (const s of servers) {
    if (s.online && s.metrics) {
      inSum += s.metrics.net_in_speed
      outSum += s.metrics.net_out_speed
    }
  }
  const sample = { ts: nowSec, in: inSum, out: outSum }
  const last = samples.at(-1)
  samples = last && last.ts === nowSec ? [...samples.slice(0, -1), sample] : [...samples, sample].slice(-MAX)
  listeners.forEach((l) => l())
}

export function resetSpeedHistory(): void {
  samples = []
  listeners.forEach((l) => l())
}

/** useSpeedHistory 汇总卡迷你曲线所用的网速历史 */
export function useSpeedHistory(): SpeedSample[] {
  return useSyncExternalStore(subscribe, getSnapshot)
}
useSpeedHistory.getSnapshot = getSnapshot
