import { act, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App.tsx'
import { clearToken, loadToken, saveToken } from './api.ts'
import { player, state } from './test/fixtures.ts'

describe('App routing', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    window.history.pushState(null, '', '/')
  })

  it('shows the create-room page at /', () => {
    render(<App />)
    expect(screen.getByRole('heading', { name: 'Poker Room' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Create room' })).toBeDisabled()
  })

  it('shows a join form for a room link without a saved session', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{"code":"ABCDEF"}', { status: 200 })))
    window.history.pushState(null, '', '/r/abcdef')
    render(<App />)
    expect(screen.getByRole('heading', { name: 'Join room ABCDEF' })).toBeInTheDocument()
  })

  it('says when a room does not exist', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}', { status: 404 })))
    window.history.pushState(null, '', '/r/NOPE42')
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Room not found' })).toBeInTheDocument()
  })
})

describe('New game with same players', () => {
  class FakeSocket {
    static all: FakeSocket[] = []
    onopen: (() => void) | null = null
    onmessage: ((ev: { data: string }) => void) | null = null
    onclose: (() => void) | null = null
    url: string
    constructor(url: string) {
      this.url = url
      FakeSocket.all.push(this)
    }
    send() {}
    close() {}
    receive(msg: object) {
      this.onmessage?.({ data: JSON.stringify(msg) })
    }
  }

  afterEach(() => {
    clearToken('OLD111')
    clearToken('NEW222')
    vi.unstubAllGlobals()
    window.history.pushState(null, '', '/')
  })

  it('moves to the new room, already seated, when the host starts one', async () => {
    FakeSocket.all = []
    vi.stubGlobal('WebSocket', FakeSocket)
    vi.stubGlobal('fetch', vi.fn().mockImplementation(async () => new Response('{"code":"X"}', { status: 200 })))
    saveToken('OLD111', 'old-token')
    window.history.pushState(null, '', '/r/OLD111')
    render(<App />)

    const old = FakeSocket.all[0]
    expect(old.url).toMatch(/\/ws\/OLD111$/)
    act(() => {
      old.receive({ type: 'welcome', playerId: 'p2' })
      old.receive({
        type: 'state',
        state: state({ code: 'OLD111', phase: 'ended', you: 'p2', players: [player({ id: 'p1', name: 'Ann' }), player({ id: 'p2', name: 'Bob' })] }),
      })
    })
    expect(window.location.pathname).toBe('/r/OLD111')

    act(() => {
      old.receive({
        type: 'state',
        state: state({ code: 'OLD111', phase: 'ended', you: 'p2', nextRoom: { code: 'NEW222', token: 'new-token' } }),
      })
    })
    expect(window.location.pathname).toBe('/r/NEW222')
    expect(loadToken('NEW222')).toBe('new-token')
    expect(FakeSocket.all.at(-1)!.url).toMatch(/\/ws\/NEW222$/)
    expect(await screen.findByText('Room NEW222')).toBeInTheDocument()
  })
})
