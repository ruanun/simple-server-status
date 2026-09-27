import '@testing-library/jest-dom/vitest'

import { cleanup } from '@testing-library/react'
import { afterEach, beforeEach, vi } from 'vitest'

import i18n from '@/i18n'
import { resetSpeedHistory } from '@/realtime/speed-history'

import { FakeWebSocket } from './fake-ws'

class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}

beforeEach(() => {
  void i18n.changeLanguage('zh-CN')
  localStorage.clear()
  vi.stubGlobal('ResizeObserver', ResizeObserverStub)
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  }))
  // Radix 组件在 jsdom 中依赖以下 DOM API
  Element.prototype.scrollIntoView = () => {}
  Element.prototype.hasPointerCapture = () => false
  Element.prototype.releasePointerCapture = () => {}
  FakeWebSocket.reset()
  vi.stubGlobal('WebSocket', FakeWebSocket)
  resetSpeedHistory()
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})
