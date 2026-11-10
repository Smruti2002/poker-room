import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { clearToken, saveToken } from '../api.ts'
import { player, state } from '../test/fixtures.ts'
import { FinalStandings } from './FinalStandings.tsx'

const ended = state({
  code: 'ABCDEF',
  phase: 'ended',
  players: [player({ id: 'p1', name: 'Ann' }), player({ id: 'p2', name: 'Bob' })],
  leaderboard: [
    { id: 'p1', name: 'Ann', profit: 0 },
    { id: 'p2', name: 'Bob', profit: 0 },
  ],
})

describe('download hand history', () => {
  afterEach(() => {
    clearToken('ABCDEF')
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('downloads the file, proving who you are with your session token in a header', async () => {
    saveToken('ABCDEF', 'tok')
    const fetch = vi.fn().mockResolvedValue(new Response('{"room":"ABCDEF","hands":[]}', { status: 200 }))
    vi.stubGlobal('fetch', fetch)
    URL.createObjectURL = vi.fn().mockReturnValue('blob:x')
    URL.revokeObjectURL = vi.fn()
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    render(<FinalStandings state={ended} send={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Download hand history' }))
    await waitFor(() => expect(click).toHaveBeenCalled())
    const [url, init] = fetch.mock.calls[0]
    expect(url).toBe('/api/rooms/ABCDEF/history')
    expect(url).not.toContain('tok')
    expect(init.headers).toEqual({ Authorization: 'Bearer tok' })
  })

  it("says why when it can't", async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(new Response('{"error":"the hand history can be downloaded once the game has ended"}', { status: 409 })),
    )
    render(<FinalStandings state={ended} send={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Download hand history' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('once the game has ended')
    expect(screen.getByRole('button', { name: 'Download hand history' })).toBeEnabled()
  })
})
