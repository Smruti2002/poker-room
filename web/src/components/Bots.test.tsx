import { fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { player, settings, state } from '../test/fixtures.ts'
import { Lobby } from './Lobby.tsx'
import { Table } from './Table.tsx'

const withBot = (over = {}) =>
  state({
    players: [player({ id: 'p1', name: 'Ann' }), player({ id: 'p2', name: 'Bob' }), player({ id: 'p3', name: 'Bot 1', bot: true })],
    ...over,
  })

describe('bots', () => {
  afterEach(() => vi.restoreAllMocks())

  it('the host adds and removes bots in the lobby', () => {
    const send = vi.fn()
    render(<Lobby state={withBot()} onSave={vi.fn()} onStart={vi.fn()} send={send} />)
    expect(screen.getByText('Bot 1').closest('li')).toHaveTextContent('🤖 bot')
    fireEvent.click(screen.getByRole('button', { name: '🤖 Add bot' }))
    expect(send).toHaveBeenCalledWith({ type: 'addBot' })
    fireEvent.click(screen.getByRole('button', { name: 'Remove Bot 1' }))
    expect(send).toHaveBeenCalledWith({ type: 'removeBot', target: 'p3' })
    // Bots aren't voted on: the host removes them.
    expect(screen.queryByRole('button', { name: 'Vote to remove Bot 1' })).toBeNull()
  })

  it('other players can neither add nor remove bots', () => {
    render(<Lobby state={withBot({ you: 'p2' })} onSave={vi.fn()} onStart={vi.fn()} send={vi.fn()} />)
    expect(screen.queryByRole('button', { name: '🤖 Add bot' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Remove Bot 1' })).toBeNull()
  })

  it('no Add bot when the room is full', () => {
    render(<Lobby state={withBot({ settings: { ...settings, maxPlayers: 3 } })} onSave={vi.fn()} onStart={vi.fn()} send={vi.fn()} />)
    expect(screen.queryByRole('button', { name: '🤖 Add bot' })).toBeNull()
  })

  it('mid-game: bots are marked on their seat, and removing one asks first', () => {
    const send = vi.fn()
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const s = withBot({
      phase: 'playing',
      stage: 'result',
      players: [player({ id: 'p1', name: 'Ann' }), player({ id: 'p2', name: 'Bob' }), player({ id: 'p3', name: 'Bot 1', bot: true })],
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={send} />)
    const seat = screen.getByLabelText('Seat Bot 1')
    expect(within(seat).getByLabelText('bot')).toBeInTheDocument()
    fireEvent.click(within(seat).getByRole('button', { name: 'Remove Bot 1' }))
    expect(confirm).toHaveBeenCalled()
    expect(send).toHaveBeenCalledWith({ type: 'removeBot', target: 'p3' })
    fireEvent.click(screen.getByRole('button', { name: '🤖 Add bot' }))
    expect(send).toHaveBeenCalledWith({ type: 'addBot' })
  })
})
