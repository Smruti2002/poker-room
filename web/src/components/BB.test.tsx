import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { HandView } from '../protocol/types.ts'
import { roundSteps } from '../roundSteps.ts'
import { player, state } from '../test/fixtures.ts'
import { DrawPicker } from './DrawPicker.tsx'
import { Table } from './Table.tsx'

const hand = (over: Partial<HandView>): HandView => ({
  number: 4, variant: 'bb', street: 'Pre-flop', boards: [], pot: 60, currentBet: 20, done: false, ...over,
})

describe('BB', () => {
  it('exchange: tap the cards to give back, then confirm', () => {
    const onAct = vi.fn()
    render(<DrawPicker hole={['As', 'Ks', '7d', '2c', '3h']} round={1} onAct={onAct} />)
    expect(screen.getByRole('button', { name: 'Keep all my cards' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Two of clubs' }))
    fireEvent.click(screen.getByRole('button', { name: 'Three of hearts' }))
    fireEvent.click(screen.getByRole('button', { name: 'Seven of diamonds' }))
    fireEvent.click(screen.getByRole('button', { name: 'Seven of diamonds' })) // changed their mind
    fireEvent.click(screen.getByRole('button', { name: 'Exchange 2 cards' }))
    expect(onAct).toHaveBeenCalledWith({ type: 'draw', cards: ['2c', '3h'] })
  })

  it('during an exchange: your picker, who we wait for, and no betting', () => {
    const s = state({
      phase: 'playing',
      stage: 'hand',
      players: [
        player({ id: 'p1', name: 'Ann', inHand: true, holeCount: 5, hole: ['As', 'Ks', '7d', '2c', '3h'], exchanging: true }),
        player({ id: 'p2', name: 'Bob', inHand: true, holeCount: 5, exchanging: true }),
        player({ id: 'p3', name: 'Cy', inHand: true, holeCount: 5 }),
      ],
      hand: hand({ street: 'Flop', drawing: true, drawRound: 1, legal: { draw: true, fold: false, check: false, call: false, callUnits: 0, bet: false, raise: false, minTo: 0, maxTo: 0 } }),
    })
    const { rerender } = render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(screen.getByRole('group', { name: 'Exchange cards' })).toHaveTextContent('Exchange 2 of 3')
    expect(screen.queryByRole('button', { name: 'Check' })).toBeNull()
    expect(screen.getByRole('status')).toHaveTextContent('Exchange 2: choose cards to give back')

    rerender(<Table state={{ ...s, players: s.players.map((p) => ({ ...p, exchanging: p.id === 'p2' })), hand: { ...s.hand!, legal: undefined } }} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(screen.getByRole('status')).toHaveTextContent('Waiting for Bob to exchange')
  })

  it('shows how many cards each player exchanged last time', () => {
    const s = state({
      phase: 'playing',
      stage: 'hand',
      players: [
        player({ id: 'p1', name: 'Ann', inHand: true, holeCount: 5, hole: ['As', 'Ks', '7d', '2c', '3h'], draws: [2, 0] }),
        player({ id: 'p2', name: 'Bob', inHand: true, holeCount: 5, draws: [3] }),
      ],
      hand: hand({ street: 'Turn', drawRound: 2, toAct: 'p2' }),
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(within(screen.getByLabelText('Seat Ann')).getByText('kept all')).toBeInTheDocument()
    expect(within(screen.getByLabelText('Seat Bob')).getByText('took 3')).toBeInTheDocument()
  })

  it('round steps: four bets with three exchanges between them', () => {
    const show = (h: HandView) =>
      roundSteps(h)
        .map((s) => (s.state === 'current' ? `[${s.label}]` : s.label))
        .join(' · ')
    expect(show(hand({}))).toBe('[Bet 1] · Exchange 1 · Bet 2 · Exchange 2 · Bet 3 · Exchange 3 · Bet 4 · Showdown')
    expect(show(hand({ drawing: true, drawRound: 0 }))).toContain('[Exchange 1]')
    expect(show(hand({ street: 'Flop', drawRound: 1 }))).toContain('[Bet 2]')
    expect(show(hand({ street: 'Turn', drawing: true, drawRound: 2 }))).toContain('[Exchange 3]')
    expect(show(hand({ street: 'River', drawRound: 3 }))).toContain('[Bet 4]')
    expect(show(hand({ street: 'Showdown', done: true }))).toContain('[Showdown]')
  })
})
