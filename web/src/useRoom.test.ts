import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, saveToken } from './api.ts'
import { useRoom } from './useRoom.ts'

/** A stand-in for the browser WebSocket that tests open and close by hand. */
class FakeSocket {
  static all: FakeSocket[] = []
  onopen: (() => void) | null = null
  onmessage: ((ev: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  sent: string[] = []
  constructor() {
    FakeSocket.all.push(this)
  }
  send(data: string) {
    this.sent.push(data)
  }
  close() {
    this.onclose?.()
  }
  /** The server accepts the connection and welcomes the player. */
  welcome() {
    this.onopen?.()
    this.onmessage?.({ data: JSON.stringify({ type: 'welcome', playerId: 'p1' }) })
  }
}

function setVisibility(state: 'visible' | 'hidden') {
  Object.defineProperty(document, 'visibilityState', { value: state, configurable: true })
  document.dispatchEvent(new Event('visibilitychange'))
}

describe('useRoom reconnecting', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    FakeSocket.all = []
    vi.stubGlobal('WebSocket', FakeSocket)
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{"code":"ABCDEF"}', { status: 200 })))
    saveToken('ABCDEF', 'tok')
  })
  afterEach(() => {
    clearToken('ABCDEF')
    setVisibility('visible')
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  // Connect, then lose the connection: a backoff timer is now pending.
  function dropped() {
    const hook = renderHook(() => useRoom('ABCDEF'))
    FakeSocket.all[0].welcome()
    act(() => FakeSocket.all.at(-1)!.close())
    expect(hook.result.current.status).toBe('reconnecting')
    return hook
  }

  it('retries straight away when the page becomes visible again', () => {
    dropped()
    setVisibility('hidden') // phone locked: no point trying yet
    expect(FakeSocket.all).toHaveLength(1)
    act(() => setVisibility('visible'))
    expect(FakeSocket.all).toHaveLength(2)
    expect(FakeSocket.all[1].sent).toHaveLength(0) // not open yet
    FakeSocket.all[1].onopen?.()
    expect(JSON.parse(FakeSocket.all[1].sent[0])).toEqual({ type: 'resume', token: 'tok' })
  })

  it('retries straight away when the network comes back', () => {
    dropped()
    act(() => {
      window.dispatchEvent(new Event('online'))
    })
    expect(FakeSocket.all).toHaveLength(2)
  })

  it('does not open a second connection while one is fine', () => {
    renderHook(() => useRoom('ABCDEF'))
    FakeSocket.all[0].welcome()
    act(() => {
      setVisibility('visible')
      window.dispatchEvent(new Event('online'))
    })
    expect(FakeSocket.all).toHaveLength(1)
  })

  it('does not reconnect twice when the backoff timer fires after an instant retry', () => {
    dropped()
    act(() => setVisibility('visible'))
    act(() => {
      vi.advanceTimersByTime(10_000)
    })
    expect(FakeSocket.all).toHaveLength(2)
  })

  it('stops listening once the page leaves the room', () => {
    const { unmount } = dropped()
    unmount()
    act(() => {
      window.dispatchEvent(new Event('online'))
    })
    expect(FakeSocket.all).toHaveLength(1)
  })
})
